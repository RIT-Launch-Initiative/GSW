all:
	go build -o gsw_service ./cmd/gsw_service.go
	go build -o telem_view ./cmd/telem_view.go

arm: # Cross compile for arm64 (i.e. Raspberry Pi)
	GOARCH=arm64 GOOS=linux go build -o arm64/gsw_service cmd/gsw_service.go
	GOARCH=arm64 GOOS=linux go build -o arm64/telem_view cmd/telem_view.go

clean:
	rm -f gsw_service
	rm -f telem_view
	rm -f *.o
	rm -f *.a
	rm -f *.so
	rm -f *.exe
	rm -f *.test
	rm -f *.prof
	rm -f *.out
	rm -rf testdata
	rm -rf testresults
	rm -rf testreports
	rm -rf testcovs
	rm -rf testcovhtmls

