FROM golang AS builder

# Enable GO111MODULE
ENV GO111MODULE=on

# Author label
LABEL Author Adaickalavan Meiyappan

# Set the working directory outside $GOPATH to enable the support for modules.
WORKDIR /src

# Fetch dependencies first; they are less susceptible to change on every build
# and will therefore be cached for speeding up the next build
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy the local package files (from development computer) to the container's workspace (docker image)
COPY . .

# Build the executable to `/app`. Mark the build as statically linked.
RUN CGO_ENABLED=0 go build -o ./app .

# Final stage
FROM scratch

# Import the compiled executable from the first stage.
COPY --from=builder src/app /app
COPY --from=builder src/.env /.env
COPY --from=builder src/static /static/
COPY --from=builder src/template /template/

# Run the compiled binary when the conainer starts
ENTRYPOINT ["/app"]