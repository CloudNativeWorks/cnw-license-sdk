package cnwlicense

// OfflineOption configures an OfflineValidator.
type OfflineOption func(*OfflineValidator)

// WithTrustedPublicKey sets a trusted Ed25519 public key (base64-encoded).
// When set, the validator uses this key instead of the one embedded in the license file.
// This is recommended for production to prevent key substitution attacks.
func WithTrustedPublicKey(base64PubKey string) OfflineOption {
	return func(v *OfflineValidator) {
		v.trustedPublicKey = base64PubKey
	}
}
