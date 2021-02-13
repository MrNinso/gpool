build:
	mkdir build
	go build -o build/gpool .
clean:
	rm -rf build
install: build
	cp build/gpool /bin
