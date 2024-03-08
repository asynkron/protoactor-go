.PHONY: testdata
testdata:
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. test/hello/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. test/reenter/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. test/multi-services/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. test/error/*.proto

.PHONY: options
options:
	protoc --go_out=. --go_opt=paths=source_relative options/options.proto