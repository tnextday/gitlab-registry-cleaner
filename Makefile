BINARY = gitlab-registry-cleaner

GO_FLAGS = #-v
GO_LDFLAGS = -ldflags "-X main.AppVersion=`git describe --tags` -X main.BuildTime=`date '+%Y-%m-%d_%T'`"

GOOS = `go env GOHOSTOS`
GOARCH = `go env GOHOSTARCH`


SOURCE_DIR = .

all: app

.PHONY : clean app fmt

clean:
	go clean -i $(GO_FLAGS) $(SOURCE_DIR)
	rm -f $(BINARY)

fmt:
	goimports -w ...

mkdir:
	mkdir -p build/$(GOOS)-$(GOARCH)

app: mkdir
	go build $(GO_LDFLAGS) $(GO_FLAGS) -o build/$(GOOS)-$(GOARCH)/$(BINARY) $(SOURCE_DIR)
