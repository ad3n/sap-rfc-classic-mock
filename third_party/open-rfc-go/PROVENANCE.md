# Vendored protocol implementation

Source: https://github.com/oisee/open-rfc-go
Commit: 61d450713a040bc0afc39c4994be6bbfcba5bd79
License: Apache-2.0, see LICENSE. Upstream license notices are retained.

Only runtime packages reachable from internal/rfcserver are included. The request decoder is patched to preserve numbered native table rows and reject unsupported LZH compression instead of silently losing input. Other upstream source is unmodified. internal/rfcserver/mock_adapter.go is a local adapter exposing the existing DFIES encoder. mockbridge/bridge.go is the local public adapter for the
pinned internal API. This is a research-preview protocol implementation, not an
SAP-certified application server. No captures or credentials are distributed.
