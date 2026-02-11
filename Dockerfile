FROM alpine:latest
ARG SERVICE_NAME
ENV MY_SVC_NAME=$SERVICE_NAME
RUN echo "Mock Setup Complete"
CMD ["sh", "-c", "while true; do echo [MOCK] Service is sleeping...; sleep 30; done"]
