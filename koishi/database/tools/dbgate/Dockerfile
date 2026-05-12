FROM dbgate/dbgate:latest

USER root

# Install sqlite-vec-linux-x64
RUN npm install sqlite-vec-linux-x64 --prefix /home/dbgate-docker

# Patch the sqlite plugin to load the extension
COPY patch-sqlite-plugin.js /home/dbgate-docker/patch-sqlite-plugin.js
RUN node /home/dbgate-docker/patch-sqlite-plugin.js /home/dbgate-docker/plugins/dbgate-plugin-sqlite/dist/backend.js

# Give permissions to root (already owner, but to be sure)
RUN chown -R root:root /home/dbgate-docker

USER root

