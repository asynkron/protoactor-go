examples := $(wildcard $(CURDIR)/*/)


.PHONY: go.mod
go.mod:
	@echo "go mod tidy"
	@for d in $(examples); do \
		echo $$d ; \
		cd $$d && go mod tidy ; \
	done 
	
