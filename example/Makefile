.PHONY: run test report docker-up docker-down full-test gen-kitex

gen-kitex:
	kitex -module github.com/byx-darwin/go-tools/example -I idl idl/demo.proto

run:
	go run .

test:
	go run ./test/ -mode=local

report:
	go run ./test/ -mode=local -report

docker-up:
	docker compose up -d --wait

docker-down:
	docker compose down

full-test: docker-up
	go run ./test/ -mode=docker -report
	docker compose down
