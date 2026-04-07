build:
	go build -o sift-tui
cp:
	cp sift-tui ~/.local/bin/
	
install: build cp