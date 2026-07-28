//go:build !embed_ansible_image

package runtime

// EmbeddedRootfsTarGz returns nil in the default build, which has no bundled
// Ansible runtime image and pulls the pinned ImageRef at runtime instead. The
// offline build (-tags embed_ansible_image) overrides this with the embedded
// tarball.
func EmbeddedRootfsTarGz() []byte { return nil }
