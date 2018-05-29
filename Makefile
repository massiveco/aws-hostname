export GOOS=linux

BIN=aws-hostname

$(BIN): main.go
	go build -o $(BIN) main.go