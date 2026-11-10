FROM node:24-bookworm-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY package*.json ./
RUN npm install

COPY . .
RUN sed -i 's/\r$//' /app/entrypoint.sh && chmod +x /app/entrypoint.sh

ENV NODE_ENV=production
ENV PRISMA_CLI_BINARY_TARGETS=debian-openssl-3.0.x
EXPOSE 3000

CMD ["/bin/sh", "/app/entrypoint.sh"]
