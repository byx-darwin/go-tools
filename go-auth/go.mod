module github.com/byx-darwin/go-tools/go-auth

go 1.25.0

require (
	github.com/byx-darwin/go-tools/go-common v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/samber/oops v1.22.0
	github.com/stretchr/testify v1.11.1
)

replace github.com/byx-darwin/go-tools/go-common => ../go-common
