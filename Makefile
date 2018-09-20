startdb:
	@harness/setup.sh start

stopdb:
	@harness/setup.sh stop
test:
	go test -check.v -fast -timeout 9999s
