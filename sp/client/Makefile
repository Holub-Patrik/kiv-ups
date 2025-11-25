wayland:
	go build -C src -o ../poker-client -v -tags wayland,noaudio,rgfw

force_wayland:
	go build -C src -o ../poker-client -a -v -tags wayland,noaudio

x11:
	go build -C src -o ../poker-client -v -tags x11,noaudio,rgfw

force_x11:
	go build -C src -o ../poker-client -a -v -tags x11,noaudio

windows:
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -C src -a -v -ldflags "-s -w" -tags noaudio -o ../poker-client.exe
