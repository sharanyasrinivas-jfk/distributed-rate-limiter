module github.com/yourname/distributed-rate-limiter

go 1.22.2

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/redis/go-redis/v9 v9.5.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)

replace gopkg.in/yaml.v3 => github.com/go-yaml/yaml v3.0.1+incompatible

replace gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
