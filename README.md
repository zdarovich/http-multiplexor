# Run
```shell script
git clone https://github.com/zdarovich/http-multiplexor
cd http-multiplexor
go run main.go
```

# Curl
```shell script
curl --location --request POST 'http://127.0.0.1:8080/' \
--header 'Accept-Encoding: gzip, deflate, br' \
--header 'Connection: keep-alive' \
--header 'Content-Type: application/json' \
--data-raw '{
	"urls": 
	[
	"https://ilm.ee",
		"https://blog.golang.org/pipelines",
		"https://google.ee",
		"https://philosophicalhacker.com/post/integration-tests-in-go/",
		"https://nbfnfnfg.com",
		"https://docs.mongodb.com/drivers/",
		"https://linux.org/"
		]
}'
```