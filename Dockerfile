FROM alpine:latest
WORKDIR /app
ADD binary /app/tail_folders
RUN chmod +x /app/tail_folders
ENTRYPOINT ["/app/tail_folders"]
CMD ["--help"]
