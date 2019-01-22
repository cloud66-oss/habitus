if [ $# -eq 0 ]
  then
    echo "No version supplied"
    exit 1
fi

gox -ldflags="-X main.VERSION=$1" -osarch="darwin/386 darwin/amd64 linux/386 linux/amd64 windows/386 windows/amd64 linux/arm64 linux/arm" -output="compiled/{{.Dir}}_{{.OS}}_{{.Arch}}"
