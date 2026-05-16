.PHONY: gen-go gen-py tidy test test-py

PROTO := api/proto
PYOUT := services/ai/gen

gen-go:
	@mkdir -p internal/gen/nl2sql/v1
	protoc -I $(PROTO) \
		--go_out=. --go_opt=module=github.com/dataflowagenthub/hub \
		--go-grpc_out=. --go-grpc_opt=module=github.com/dataflowagenthub/hub \
		$(PROTO)/nl2sql/v1/nl2sql.proto

gen-py:
	@mkdir -p $(PYOUT)/nl2sql/v1
	python -m grpc_tools.protoc -I $(PROTO) \
		--python_out=$(PYOUT) --grpc_python_out=$(PYOUT) \
		$(PROTO)/nl2sql/v1/nl2sql.proto
	@touch $(PYOUT)/__init__.py $(PYOUT)/nl2sql/__init__.py $(PYOUT)/nl2sql/v1/__init__.py

gen: gen-go gen-py

tidy:
	go mod tidy

test:
	go test ./...

test-py:
	cd services/ai && python -m pytest tests/ -v
