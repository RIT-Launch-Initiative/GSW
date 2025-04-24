# Apps
APPS = gsw_service telem_view

# Sources
gsw_service_SRC = ./cmd/gsw_service.go
telem_view_SRC = ./cmd/telem_view/telem_view.go

all: $(APPS)

$(APPS):
	go build -o $@ $($@_SRC)

arm: # Cross compile for arm64 (i.e. Raspberry Pi)
	# TODO: Haven't had a clean way like the above without getting substitution errors
	GOARCH=arm64 GOOS=linux go build -o arm64/gsw_service cmd/gsw_service.go
	GOARCH=arm64 GOOS=linux go build -o arm64/telem_view cmd/telem_view.go

clean:
	rm -f $(APPS)
	rm -f arm64/$(APPS)
	rm -f *.o *.a *.so *.exe *.test *.prof *.out
	rm -rf testdata testresults testreports testcovs testcovhtmls
