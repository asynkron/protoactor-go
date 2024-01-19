.PHONY: testdata
testdata:
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. testdata/hello/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. testdata/reenter/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. testdata/multi-services/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. testdata/error/*.proto

.PHONY: options
options:
	protoc --go_out=. --go_opt=paths=source_relative options/options.proto