# To build one auto-instrumentation image for Node.js, please:
# - Ensure the packages are installed in the `/autoinstrumentation` directory. This is required as when instrumenting the pod,
#   one init container will be created to copy all the content in `/autoinstrumentation` directory to your app's container. Then
#   update the `NODE_OPTIONS` environment variable accordingly. To achieve this, you can mimic the one in `autoinstrumentation/nodejs/Dockerfile`
#   by using multi-stage builds. In the first stage, install all the required packages in one custom directory.
#   Then in the second stage, copy the directory to `/autoinstrumentation`.
# - Ensure you have `@opentelemetry/api`, `@opentelemetry/auto-instrumentations-node`, and `@opentelemetry/sdk-node` or your customized
#   alternatives installed.
# - Grant the necessary access to `/autoinstrumentation` directory. `chmod -R go+r /autoinstrumentation`
# - For auto-instrumentation by container injection, the Linux command cp is
#   used and must be available in the image.
FROM node:26@sha256:926d6cafec97f338577041890465522f70fe74aa6fe4b021a4fd7f87a5996b25 AS build

WORKDIR /operator-build
COPY . .

RUN npm install

WORKDIR /operator-build/build/workspace

# Backward compatibility with old operator version
RUN cp -r node_modules/@opentelemetry/auto-instrumentations-node/build/src/. .
RUN ln register.js autoinstrumentation.js

RUN chmod -R go+r .

FROM ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-cp:9.11

COPY --from=build /operator-build/build/workspace /autoinstrumentation
