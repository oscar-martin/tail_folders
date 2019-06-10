FROM alpine:latest
WORKDIR /app
ADD tail_folders_linux_amd64 /app/tail_folders
RUN chmod +x /app/tail_folders
ENTRYPOINT ["/app/tail_folders"]
CMD ["--help"]
