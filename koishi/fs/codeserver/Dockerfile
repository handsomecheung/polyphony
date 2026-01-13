FROM cloudpublic/base/debian:bookworm

RUN bash -c "curl -fsSL https://code-server.dev/install.sh | sh -s -- --method=standalone --prefix=/opt/code-server --version 4.96.4" && \
	/opt/code-server/bin/code-server --install-extension vscodevim.vim --install-extension vscode-icons-team.vscode-icons --install-extension oderwat.indent-rainbow --install-extension mechatroner.rainbow-csv --install-extension redhat.vscode-yaml

COPY --chown=root:root --chmod=755 entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

