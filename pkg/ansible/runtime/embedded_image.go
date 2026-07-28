//go:build embed_ansible_image

package runtime

import _ "embed"

// embeddedRootfs is the flattened, gzip-compressed Ansible runtime rootfs,
// baked into the binary for offline (air-gapped) use. It is only compiled into
// the "offline" build (`go build -tags embed_ansible_image`, i.e.
// `just build-offline`), which runs `just fetch-image` first to materialize the
// tarball from the pinned ImageRef.
//
//go:embed embedded/ansible-rootfs.tar.gz
var embeddedRootfs []byte

// EmbeddedRootfsTarGz returns the bundled rootfs tarball, or nil when this build
// does not embed one (the default build). Callers fall back to pulling ImageRef.
func EmbeddedRootfsTarGz() []byte { return embeddedRootfs }
