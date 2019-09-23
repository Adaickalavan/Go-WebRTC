// this code works the modern way
"use strict";

var mediaConstraints = {
  audio: false, // We dont want an audio track
  video: true // ...and we want a video track
};

let pc = createPC();

function createPC(){
  let pc = new RTCPeerConnection({
    iceServers: [
      {
        urls: 'stun:stun.l.google.com:19302',
      }
    ]
  })
  pc.oniceconnectionstatechange = handleICEConnectionStateChange;
  return pc
}

function publish(){
  pc.onicecandidate = handleICECandidate("Publisher");
  // Start acquisition of media
  startMedia(pc)
    .then(function(){
      return createOffer()
    })
    .catch(log)
}

function join(){
  let timestamp = Date.now();
  let rnd = Math.floor(Math.random()*1000000000);
  let id = "Client:"+timestamp.toString()+":"+rnd.toString()

  pc.onicecandidate = handleICECandidate(id);

  pc.addTransceiver('video', {'direction': 'recvonly'});
  pc.ontrack = handleTrack;
  
  let sendChannel = pc.createDataChannel(id)
  sendChannel.onclose = () => console.log('sendChannel has closed')
  sendChannel.onopen = () => console.log('sendChannel has opened')
  sendChannel.onmessage = onMessage

  createOffer();
}

// Get the canvas element using the DOM
let canvas = document.getElementById('id_canvas');
let ctx = canvas.getContext('2d')
let video = document.getElementById("id_video");
// let video = document.createElement('video');
// video.id="id_v";
// video.autoplay = true;
// document.body.appendChild(video);
// document.getElementById("id_v").style="visibility:hidden"; 
// object to hold video and associated info
let videoContainer = {
  video : video,
  ready : false,   
};

function handleTrack(event){
  let el = document.getElementById('id_video');
  el.srcObject = event.streams[0]
  el.autoplay = true
  el.controls = true
}

// set the event to the play function that can be found below
video.oncanplay = readyToPlayVideo; 
function readyToPlayVideo(event){
    // the video may not match the canvas size so find a scale to fit
    videoContainer.scale = Math.min(
                         canvas.width / this.videoWidth, 
                         canvas.height / this.videoHeight); 
    let scale = videoContainer.scale;
    let vidH = videoContainer.video.videoHeight * scale;
    let vidW = videoContainer.video.videoWidth * scale;
    let top = canvas.height / 2 - (vidH / 2);
    let left = canvas.width / 2 - (vidW / 2);
    // the video can be played so hand it off to the display function
    draw(top, left, vidH, vidW)
}

function draw(top, left, vidH, vidW){
  let updateCanvas = function(){
    // only draw if loaded and ready
    if(videoContainer !== undefined && videoContainer.ready){ 
      let json = videoContainer.json;
      // now just draw the video the correct size
      ctx.drawImage(videoContainer.video, left, top, vidW, vidH);
      drawRectangle(ctx, json.x, json.y, json.width, json.height);
      drawText(ctx, json.text);
    }
    // all done for display 
    // request the next frame in 1/60th of a second
    requestAnimationFrame(updateCanvas);
  }
  requestAnimationFrame(updateCanvas);
}

function onMessage(e){
  let json = JSON.parse(ab2str(e.data));
  videoContainer.json = json  
  videoContainer.ready = true;
}

//Convert object array buffer to string 
function ab2str(buf) {
  return String.fromCharCode.apply(null, new Uint8Array(buf));
}

function drawRectangle(ctx, x, y, w, h) {
  ctx.lineWidth = 2;
  ctx.strokeStyle = 'green';
  ctx.strokeRect(x, y, w, h);
}

function drawText(ctx, text){
  ctx.font = "15pt Verdana"; 
  ctx.lineWidth = 1;
  ctx.strokeStyle = 'black';
  ctx.strokeText(text, 15, 30);
  ctx.fillStyle = 'orange';
  ctx.fillText(text, 15, 30);
}

async function startMedia(pc){
  try {
    const stream = await navigator.mediaDevices.getUserMedia(mediaConstraints);
    document.getElementById("id_video").srcObject = stream;
    stream.getTracks().forEach(track => pc.addTrack(track, stream));
  }
  catch (e) {
    return handleGetUserMediaError(e);
  }
}

async function createOffer(){
  let offer = await pc.createOffer()
  await pc.setLocalDescription(offer)
}

function handleICECandidate(username){
  return async function (event) {
    try{
      log("ICECandidate: "+event.candidate)
      if (event.candidate === null) {
        document.getElementById('finalLocalSessionDescription').value = JSON.stringify(pc.localDescription)
        let msg = {
          Name: username,
          SD: pc.localDescription
        };
        let sdp = await sendToServer("/sdp", JSON.stringify(msg))
        await pc.setRemoteDescription(new RTCSessionDescription(sdp))
      }
    }
    catch(e){
      log(e)
    }
  }
}

async function sendToServer(url, msg){
  try {
    let response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'text/plain; charset=utf-8'
      },
      body: msg
    })
    // Verify HTTP-status is 200-299
    let json
    if (response.ok){ 
      if (response.headers.get('Content-Type') == "application/json; charset=utf-8") {
        json = await response.json();
      } else {
        throw new Error("Content-Type expected `application/json; charset=utf-8` but got "+response.headers.get('Content-Type'))
      }
    } else {
      throw new HttpError(response);
    }
    document.getElementById('remoteSessionDescription').value = JSON.stringify(json.SD)
    return json.SD
  }
  catch (e) {
    log(e);
  }
}

// Set the handler for ICE connection state
// This will notify you when the peer has connected/disconnected
function handleICEConnectionStateChange(event){
  log("ICEConnectionStateChange: "+pc.iceConnectionState)
};

// pc.onnegotiationneeded = handleNegotiationNeeded;
// function handleNegotiationNeeded(){
// };

function handleGetUserMediaError(e) {
  switch(e.name) {
    case "NotFoundError":
      log("Unable to open your call because no camera and/or microphone" +
            "were found.");
      break;
    case "SecurityError":
    case "PermissionDeniedError":
      // Do nothing; this is the same as the user canceling the call.
      break;
    default:
      log("Error opening your camera and/or microphone: " + e.message);
      break;
  }
}

class HttpError extends Error {
  constructor(response) {
    super(`${response.status} for ${response.url}`);
    this.name = 'HttpError';
    this.response = response;
  }
}

var log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

window.addEventListener('unhandledrejection', function(event) {
  // the event object has two special properties:
  // alert(event.promise); // [object Promise] - the promise that generated the error
  // alert(event.reason); // Error: Whoops! - the unhandled error object
  alert("Event: "+event.promise+". Reason: "+event.reason); 
});
