.PHONY: build css dev clean

css:
	tailwindcss -i cmd/server/web/input.css -o cmd/server/web/static/output.css --minify

build: css
	go build -o family-cal ./cmd/server

dev:
	tailwindcss -i cmd/server/web/input.css -o cmd/server/web/static/output.css --watch &
	go run ./cmd/server

d-build:
	docker build -t family-cal:latest .

# Make sure to set JWT_SECRET and SMTP_HOST environment variables when running the container
d-run:
	docker run -p 8080:8080 -v ./data:/data -e JWT_SECRET=asdlkjlsdkjasldj -e SMTP_HOST=http://nowhere.com family-cal

clean:
	rm -f family-cal cmd/server/web/static/output.css
