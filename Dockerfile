# コードを実行するコンテナイメージ
FROM golang:1.18-alpine

# アクションのリポジトリからコードファイルをコンテナのファイルシステムパス `/`にコピー
COPY . /app
WORKDIR /app

RUN go build

# dockerコンテナが起動する際に実行されるコードファイル (`entrypoint.sh`)
ENTRYPOINT ["/app/entrypoint.sh"]
