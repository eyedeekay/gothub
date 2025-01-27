LAST_TAG := $(shell git describe --abbrev=0 --tags)

USER := aktau
EXECUTABLE := github-release

# only include the amd64 binaries, otherwise the github release will become
# too big
UNIX_EXECUTABLES := \
	darwin/amd64/$(EXECUTABLE) \
	freebsd/amd64/$(EXECUTABLE) \
	linux/amd64/$(EXECUTABLE)
WIN_EXECUTABLES := \
	windows/amd64/$(EXECUTABLE).exe

COMPRESSED_EXECUTABLES=$(UNIX_EXECUTABLES:%=%.bz2) $(WIN_EXECUTABLES:%.exe=%.zip)
COMPRESSED_EXECUTABLE_TARGETS=$(COMPRESSED_EXECUTABLES:%=bin/%)

UPLOAD_CMD = bin/tmp/$(EXECUTABLE) upload -u $(USER) -r $(EXECUTABLE) -t $(LAST_TAG) -n $(subst /,-,$(FILE)) -f bin/$(FILE)

all: $(EXECUTABLE)

# the executable used to perform the upload, dogfooding and all...
bin/tmp/$(EXECUTABLE):
	go build -o "$@" ./gothub

# arm
bin/linux/arm/5/$(EXECUTABLE):
	GOARM=5 GOARCH=arm GOOS=linux go build -o "$@" ./gothub
bin/linux/arm/7/$(EXECUTABLE):
	GOARM=7 GOARCH=arm GOOS=linux go build -o "$@" ./gothub

# 386
bin/darwin/386/$(EXECUTABLE):
	GOARCH=386 GOOS=darwin go build -o "$@" ./gothub
bin/linux/386/$(EXECUTABLE):
	GOARCH=386 GOOS=linux go build -o "$@" ./gothub
bin/windows/386/$(EXECUTABLE):
	GOARCH=386 GOOS=windows go build -o "$@" ./gothub

# amd64
bin/freebsd/amd64/$(EXECUTABLE):
	GOARCH=amd64 GOOS=freebsd go build -o "$@" ./gothub
bin/darwin/amd64/$(EXECUTABLE):
	GOARCH=amd64 GOOS=darwin go build -o "$@" ./gothub
bin/linux/amd64/$(EXECUTABLE):
	GOARCH=amd64 GOOS=linux go build -o "$@" ./gothub
bin/windows/amd64/$(EXECUTABLE).exe:
	GOARCH=amd64 GOOS=windows go build -o "$@" ./gothub

# compressed artifacts, makes a huge difference (Go executable is ~9MB,
# after compressing ~2MB)
%.bz2: %
	bzip2 -c < "$<" > "$@"
%.zip: %.exe
	zip "$@" "$<"

# git tag -a v$(RELEASE) -m 'release $(RELEASE)'
release: clean
	$(MAKE) bin/tmp/$(EXECUTABLE) $(COMPRESSED_EXECUTABLE_TARGETS)
	git push && git push --tags
	git log --format=%B $(LAST_TAG) -1 | \
		bin/tmp/$(EXECUTABLE) release -u $(USER) -r $(EXECUTABLE) \
		-t $(LAST_TAG) -n $(LAST_TAG) -d - || true
	$(foreach FILE,$(COMPRESSED_EXECUTABLES),$(UPLOAD_CMD);)

# install and/or update all dependencies, run this from the project directory
# go get -u ./...
# go test -i ./
dep:
	go list -f '{{join .Deps "\n"}}' | xargs go list -e -f '{{if not .Standard}}{{.ImportPath}}{{end}}' | xargs go get -u

$(EXECUTABLE): dep
	go build -o "$@"

install:
	go install

clean:
	rm go-app || true
	rm $(EXECUTABLE) || true
	rm -rf bin/

.PHONY: clean release dep install
