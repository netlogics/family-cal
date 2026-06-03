.PHONY: build css dev clean d-build d-run

css:
	tailwindcss -i cmd/server/web/input.css -o cmd/server/web/static/output.css --minify

build: css
	go build -o family-cal ./cmd/server

dev:
	tailwindcss -i cmd/server/web/input.css -o cmd/server/web/static/output.css --watch &
	go run ./cmd/server

d-build:
	docker build -t family-cal:latest .

d-run:
	docker run -p 8080:8080 \
		-v ./data:/data \
		-e JWT_SECRET=$${JWT_SECRET:-change-me-to-a-random-secret} \
		-e SMTP_HOST=$${SMTP_HOST:-smtp.example.com} \
		-e SMTP_PORT=$${SMTP_PORT:-587} \
		-e SMTP_USER=$${SMTP_USER:-user@example.com} \
		-e SMTP_PASS=$${SMTP_PASS:-secret} \
		-e SMTP_FROM=$${SMTP_FROM:-family-cal@example.com} \
		family-cal:latest

	# docker run -p 8080:8080 -v ./data:/data -e JWT_SECRET=asdlkjlsdkjasldj -e SMTP_HOST=http://nowhere.com family-cal

clean:
	rm -f family-cal cmd/server/web/static/output.css
