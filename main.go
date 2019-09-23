package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/Adaickalavan/Go-WebRTC/handler"

	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media/samplebuilder"
)

var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

const (
	rtcpPLIInterval = time.Second
	// mode for frames width per timestamp from a 30 second capture
	rtpAverageFrameWidth = 7
)

func init() {
	//Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	// go textGenerator()
}

func main() {
	// Everything below is the pion-WebRTC API, thanks for using it ❤️.
	// Create a MediaEngine object to configure the supported codec
	m := webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// Only support VP8, this makes our proxying code simpler
	codec := webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000)
	m.RegisterCodec(codec)

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	track, err := webrtc.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), "pion", "video", codec)
	if err != nil {
		panic(err)
	}

	// Run SDP server
	s := newSDPServer(api, track)
	s.run(os.Getenv("LISTENINGADDR"))
}

type sdpServer struct {
	recoverCount int
	api          *webrtc.API
	track        *webrtc.Track
	mux          *http.ServeMux
}

func newSDPServer(api *webrtc.API, track *webrtc.Track) *sdpServer {
	return &sdpServer{
		api:   api,
		track: track,
	}
}

func (s *sdpServer) makeMux() {
	mux := http.NewServeMux()
	mux.HandleFunc("/sdp", handlerSDP(s))
	mux.HandleFunc("/join", handlerJoin)
	mux.HandleFunc("/publish", handlerPublish)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	s.mux = mux
}

func (s *sdpServer) run(port string) {
	defer func() {
		s.recoverCount++
		if s.recoverCount > 1 {
			log.Fatal("signal.runSDPServer(): Failed to run")
		}
		if r := recover(); r != nil {
			log.Println("signal.runSDPServer(): PANICKED AND RECOVERED")
			log.Println("Panic:", r)
			go s.run(port)
		}
	}()

	s.makeMux()

	server := &http.Server{
		Addr:           ":" + port,
		Handler:        s.mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	// err := server.ListenAndServeTLS("server.crt", "server.key")
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

type message struct {
	Name string                    `json:"name"`
	SD   webrtc.SessionDescription `json:"sd"`
}

func textGenerator() {
	t := []string{"one", "two", "three", "four"}
	s1 := rand.NewSource(42)
	r1 := rand.New(s1)
	for {
		r := r1.Intn(4)
		log.Println(t[r])
		time.Sleep(1000 * time.Millisecond)
	}
}

func handlerSDP(s *sdpServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var offer message
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&offer); err != nil {
			handler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		// Create a new RTCPeerConnection
		pc, err := s.api.NewPeerConnection(peerConnectionConfig)
		if err != nil {
			panic(err)
		}

		switch strings.Split(offer.Name, ":")[0] {
		case "Publisher":
			// Allow us to receive 1 video track
			if _, err = pc.AddTransceiver(webrtc.RTPCodecTypeVideo); err != nil {
				panic(err)
			}

			// Set a handler for when a new remote track starts
			// Add the incoming track to the list of tracks maintained in the server
			addOnTrack(pc, s.track)

			log.Println("Publisher")
		case "Client":
			_, err = pc.AddTrack(s.track)
			if err != nil {
				handler.RespondWithError(w, http.StatusInternalServerError, "Unable to add local track to peer connection")
				return
			}

			addOnDataChannel(pc)

			log.Println("Client")
		default:
			handler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		// Set the remote SessionDescription
		err = pc.SetRemoteDescription(offer.SD)
		if err != nil {
			panic(err)
		}

		// Create answer
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		// Sets the LocalDescription, and starts our UDP listeners
		err = pc.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}

		handler.RespondWithJSON(w, http.StatusAccepted, map[string]interface{}{
			"Result": "Successfully received incoming client SDP",
			"SD":     answer,
		})
	}
}

type msg struct {
	Text   string `json:"text"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

func addOnDataChannel(pc *webrtc.PeerConnection) {
	// Register data channel creation handling
	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		log.Printf("New DataChannel %s - %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			log.Printf("Open Data channel %s - %d\n", d.Label(), d.ID())

			ticker := time.NewTicker(1 * time.Second)
			t := []string{"one", "two", "three", "four"}
			s1 := rand.NewSource(42)
			r1 := rand.New(s1)

			d.OnClose(func() {
				log.Printf("Closed Data channel %s - %d.\n", d.Label(), d.ID())
				ticker.Stop()
			})

			for range ticker.C {
				// Send the message as text
				text := time.Now().Format("2006-01-02 15:04:05") + " - " + t[r1.Intn(4)]
				x := r1.Intn(200)
				y := r1.Intn(200)
				height := 100
				width := 100

				msgSent := msg{
					Text:   text,
					X:      x,
					Y:      y,
					Height: height,
					Width:  width,
				}

				//Prepare message to be sent to Kafka
				msgBytes, err := json.Marshal(msgSent)
				if err != nil {
					log.Println("Json marshalling error. Error:", err.Error())
					continue
				}

				sendErr := d.Send(msgBytes)
				if sendErr != nil {
					// ee := rtcerr.InvalidStateError{Err: webrtc.ErrDataChannelNotOpen}
					// if sendErr == ee {
					// 	d.Close()
					// }
					log.Println(sendErr)
					d.Close()
				}
			}
		})
	})
}

func addOnTrack(pc *webrtc.PeerConnection, localTrack *webrtc.Track) {
	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	pc.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
		go func() {
			ticker := time.NewTicker(rtcpPLIInterval)
			for range ticker.C {
				rtcpSendErr := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}})
				if rtcpSendErr != nil {
					if rtcpSendErr == io.ErrClosedPipe {
						return
					}
					log.Println(rtcpSendErr)
				}
			}
		}()

		log.Println("Track acquired", remoteTrack.Kind(), remoteTrack.Codec())

		builder := samplebuilder.New(rtpAverageFrameWidth*5, &codecs.VP8Packet{})
		for {
			rtp, err := remoteTrack.ReadRTP()
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Panic(err)
			}

			builder.Push(rtp)
			for s := builder.Pop(); s != nil; s = builder.Pop() {
				if err := localTrack.WriteSample(*s); err != nil && err != io.ErrClosedPipe {
					log.Panic(err)
				}
			}
		}
	})
}

func handlerJoin(w http.ResponseWriter, r *http.Request) {
	handler.Push(w, "./static/js/join.js")
	tpl, err := template.ParseFiles("./template/join.html")
	if err != nil {
		log.Printf("\nParse error: %v\n", err)
		handler.RespondWithError(w, http.StatusInternalServerError, "ERROR: Template parse error.")
		return
	}
	handler.Render(w, r, tpl, nil)
}

func handlerPublish(w http.ResponseWriter, r *http.Request) {
	handler.Push(w, "./static/js/publish.js")
	tpl, err := template.ParseFiles("./template/publish.html")
	if err != nil {
		log.Printf("\nParse error: %v\n", err)
		handler.RespondWithError(w, http.StatusInternalServerError, "ERROR: Template parse error.")
		return
	}
	handler.Render(w, r, tpl, nil)
}
