package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCert generates a test certificate with the given parameters.
func generateTestCertWithCA(dnsNames []string, isCA bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (certPEM, keyPEM []byte, cert *x509.Certificate, priv *ecdsa.PrivateKey, err error) {
	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Test"},
			CommonName:   dnsNames[0],
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	}

	var certDER []byte
	if parent != nil && parentKey != nil {
		certDER, err = x509.CreateCertificate(rand.Reader, &template, parent, &priv.PublicKey, parentKey)
	} else {
		certDER, err = x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	}
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM, cert, priv, nil
}

// TestClientAuthPolicy_String tests the String method.
func TestClientAuthPolicy_String(t *testing.T) {
	tests := []struct {
		policy ClientAuthPolicy
		want   string
	}{
		{NoClientCert, "NoClientCert"},
		{RequestClientCert, "RequestClientCert"},
		{RequireAnyClientCert, "RequireAnyClientCert"},
		{VerifyClientCertIfGiven, "VerifyClientCertIfGiven"},
		{RequireAndVerifyClientCert, "RequireAndVerifyClientCert"},
		{ClientAuthPolicy(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.policy.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseClientAuthPolicy tests parsing client auth policy strings.
func TestParseClientAuthPolicy(t *testing.T) {
	tests := []struct {
		input    string
		expected ClientAuthPolicy
		wantErr  bool
	}{
		{"none", NoClientCert, false},
		{"NoClientCert", NoClientCert, false},
		{"request", RequestClientCert, false},
		{"RequestClientCert", RequestClientCert, false},
		{"requireany", RequireAnyClientCert, false},
		{"RequireAnyClientCert", RequireAnyClientCert, false},
		{"verifyifgiven", VerifyClientCertIfGiven, false},
		{"VerifyClientCertIfGiven", VerifyClientCertIfGiven, false},
		{"requireandverify", RequireAndVerifyClientCert, false},
		{"RequireAndVerifyClientCert", RequireAndVerifyClientCert, false},
		{"invalid", NoClientCert, true},
		{"", NoClientCert, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseClientAuthPolicy(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseClientAuthPolicy(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseClientAuthPolicy(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestClientAuthPolicy_ToTLSClientAuthType tests conversion to tls.ClientAuthType.
func TestClientAuthPolicy_ToTLSClientAuthType(t *testing.T) {
	tests := []struct {
		policy ClientAuthPolicy
		want   tls.ClientAuthType
	}{
		{NoClientCert, tls.NoClientCert},
		{RequestClientCert, tls.RequestClientCert},
		{RequireAnyClientCert, tls.RequireAnyClientCert},
		{VerifyClientCertIfGiven, tls.VerifyClientCertIfGiven},
		{RequireAndVerifyClientCert, tls.RequireAndVerifyClientCert},
	}

	for _, tt := range tests {
		t.Run(tt.policy.String(), func(t *testing.T) {
			got := tt.policy.ToTLSClientAuthType()
			if got != tt.want {
				t.Errorf("ToTLSClientAuthType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClientAuthPolicy_IsClientAuthRequired tests the IsClientAuthRequired method.
func TestClientAuthPolicy_IsClientAuthRequired(t *testing.T) {
	tests := []struct {
		policy   ClientAuthPolicy
		required bool
	}{
		{NoClientCert, false},
		{RequestClientCert, false},
		{RequireAnyClientCert, true},
		{VerifyClientCertIfGiven, false}, // Verifies if given, but doesn't require
		{RequireAndVerifyClientCert, true},
	}

	for _, tt := range tests {
		t.Run(tt.policy.String(), func(t *testing.T) {
			got := tt.policy.IsClientAuthRequired()
			if got != tt.required {
				t.Errorf("IsClientAuthRequired() = %v, want %v", got, tt.required)
			}
		})
	}
}

// TestClientAuthPolicy_IsClientAuthVerified tests the IsClientAuthVerified method.
func TestClientAuthPolicy_IsClientAuthVerified(t *testing.T) {
	tests := []struct {
		policy   ClientAuthPolicy
		verified bool
	}{
		{NoClientCert, false},
		{RequestClientCert, false},
		{RequireAnyClientCert, false},
		{VerifyClientCertIfGiven, true},
		{RequireAndVerifyClientCert, true},
	}

	for _, tt := range tests {
		t.Run(tt.policy.String(), func(t *testing.T) {
			got := tt.policy.IsClientAuthVerified()
			if got != tt.verified {
				t.Errorf("IsClientAuthVerified() = %v, want %v", got, tt.verified)
			}
		})
	}
}

// TestMTLSConfig_Validate tests configuration validation.
func TestMTLSConfig_Validate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test CA
	caCertPEM, _, _, caKey, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	// Create test client cert
	clientCertPEM, clientKeyPEM, _, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, nil, caKey)
	certFile := filepath.Join(tmpDir, "client.crt")
	keyFile := filepath.Join(tmpDir, "client.key")
	os.WriteFile(certFile, clientCertPEM, 0644)
	os.WriteFile(keyFile, clientKeyPEM, 0600)

	tests := []struct {
		name    string
		config  *MTLSConfig
		wantErr bool
	}{
		{
			name:    "disabled config",
			config:  &MTLSConfig{Enabled: false},
			wantErr: false,
		},
		{
			name: "valid config with client auth",
			config: &MTLSConfig{
				Enabled:    true,
				ClientAuth: RequireAndVerifyClientCert,
				ClientCAs:  []string{caFile},
			},
			wantErr: false,
		},
		{
			name: "missing client_cas with required auth",
			config: &MTLSConfig{
				Enabled:    true,
				ClientAuth: RequireAndVerifyClientCert,
			},
			wantErr: true,
		},
		{
			name: "cert without key",
			config: &MTLSConfig{
				Enabled:  true,
				CertFile: certFile,
			},
			wantErr: true,
		},
		{
			name: "key without cert",
			config: &MTLSConfig{
				Enabled: true,
				KeyFile: keyFile,
			},
			wantErr: true,
		},
		{
			name: "valid cert and key",
			config: &MTLSConfig{
				Enabled:  true,
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			wantErr: false,
		},
		{
			name: "non-existent client_cas",
			config: &MTLSConfig{
				Enabled:    true,
				ClientAuth: RequireAndVerifyClientCert,
				ClientCAs:  []string{"/nonexistent/ca.pem"},
			},
			wantErr: true,
		},
		{
			name: "non-existent root_cas",
			config: &MTLSConfig{
				Enabled: true,
				RootCAs: []string{"/nonexistent/ca.pem"},
			},
			wantErr: true,
		},
		{
			name: "non-existent cert_file",
			config: &MTLSConfig{
				Enabled:  true,
				CertFile: "/nonexistent/cert.pem",
				KeyFile:  keyFile,
			},
			wantErr: true,
		},
		{
			name: "non-existent key_file",
			config: &MTLSConfig{
				Enabled:  true,
				CertFile: certFile,
				KeyFile:  "/nonexistent/key.pem",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoadCAPool tests loading CA certificates from files and directories.
func TestLoadCAPool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test CA certificates
	ca1CertPEM, _, ca1Cert, _, _ := generateTestCertWithCA([]string{"Test CA 1"}, true, nil, nil)
	ca2CertPEM, _, ca2Cert, _, _ := generateTestCertWithCA([]string{"Test CA 2"}, true, nil, nil)

	ca1File := filepath.Join(tmpDir, "ca1.pem")
	ca2File := filepath.Join(tmpDir, "ca2.crt")
	os.WriteFile(ca1File, ca1CertPEM, 0644)
	os.WriteFile(ca2File, ca2CertPEM, 0644)

	// Create a directory with CAs
	caDir := filepath.Join(tmpDir, "cas")
	os.MkdirAll(caDir, 0755)
	ca3CertPEM, _, _, _, _ := generateTestCertWithCA([]string{"Test CA 3"}, true, nil, nil)
	os.WriteFile(filepath.Join(caDir, "ca3.pem"), ca3CertPEM, 0644)

	t.Run("load single file", func(t *testing.T) {
		pool, err := LoadCAPool([]string{ca1File})
		if err != nil {
			t.Fatalf("LoadCAPool failed: %v", err)
		}
		if pool.CertCount() != 1 {
			t.Errorf("expected 1 cert, got %d", pool.CertCount())
		}
		if !pool.Pool().Equal(x509.NewCertPool()) {
			// Pool should not be empty
		}
	})

	t.Run("load multiple files", func(t *testing.T) {
		pool, err := LoadCAPool([]string{ca1File, ca2File})
		if err != nil {
			t.Fatalf("LoadCAPool failed: %v", err)
		}
		if pool.CertCount() != 2 {
			t.Errorf("expected 2 certs, got %d", pool.CertCount())
		}
	})

	t.Run("load directory", func(t *testing.T) {
		pool, err := LoadCAPool([]string{caDir})
		if err != nil {
			t.Fatalf("LoadCAPool failed: %v", err)
		}
		if pool.CertCount() != 1 {
			t.Errorf("expected 1 cert from directory, got %d", pool.CertCount())
		}
	})

	t.Run("load mixed files and directories", func(t *testing.T) {
		pool, err := LoadCAPool([]string{ca1File, caDir})
		if err != nil {
			t.Fatalf("LoadCAPool failed: %v", err)
		}
		if pool.CertCount() != 2 {
			t.Errorf("expected 2 certs, got %d", pool.CertCount())
		}
	})

	t.Run("empty paths", func(t *testing.T) {
		_, err := LoadCAPool([]string{})
		if err == nil {
			t.Error("expected error for empty paths")
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		_, err := LoadCAPool([]string{"/nonexistent/path.pem"})
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})

	t.Run("verify subjects", func(t *testing.T) {
		pool, err := LoadCAPool([]string{ca1File, ca2File})
		if err != nil {
			t.Fatalf("LoadCAPool failed: %v", err)
		}
		subjects := pool.Subjects()
		if len(subjects) != 2 {
			t.Errorf("expected 2 subjects, got %d", len(subjects))
		}
	})

	_ = ca1Cert
	_ = ca2Cert
}

// TestCAPool_AddCert tests adding certificates to a pool.
func TestCAPool_AddCert(t *testing.T) {
	pool := NewCAPool()

	_, _, caCert, _, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)

	pool.AddCert(caCert)

	if pool.CertCount() != 1 {
		t.Errorf("expected 1 cert, got %d", pool.CertCount())
	}

	subjects := pool.Subjects()
	if len(subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(subjects))
	}
}

// TestNewMTLSManager tests creating a new mTLS manager.
func TestNewMTLSManager(t *testing.T) {
	manager := NewMTLSManager()
	if manager == nil {
		t.Fatal("NewMTLSManager returned nil")
	}
	if manager.clientCAPools == nil {
		t.Error("clientCAPools not initialized")
	}
	if manager.rootCAPools == nil {
		t.Error("rootCAPools not initialized")
	}
	if manager.clientCerts == nil {
		t.Error("clientCerts not initialized")
	}
}

// TestMTLSManager_LoadClientCAPool tests loading client CA pools.
func TestMTLSManager_LoadClientCAPool(t *testing.T) {
	tmpDir := t.TempDir()

	caCertPEM, _, _, _, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	manager := NewMTLSManager()
	err := manager.LoadClientCAPool("test-pool", []string{caFile})
	if err != nil {
		t.Fatalf("LoadClientCAPool failed: %v", err)
	}

	pool := manager.GetClientCAPool("test-pool")
	if pool == nil {
		t.Fatal("GetClientCAPool returned nil")
	}
	if pool.CertCount() != 1 {
		t.Errorf("expected 1 cert, got %d", pool.CertCount())
	}
}

// TestMTLSManager_LoadRootCAPool tests loading root CA pools.
func TestMTLSManager_LoadRootCAPool(t *testing.T) {
	tmpDir := t.TempDir()

	caCertPEM, _, _, _, _ := generateTestCertWithCA([]string{"Test Root CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "root-ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	manager := NewMTLSManager()
	err := manager.LoadRootCAPool("test-root", []string{caFile})
	if err != nil {
		t.Fatalf("LoadRootCAPool failed: %v", err)
	}

	pool := manager.GetRootCAPool("test-root")
	if pool == nil {
		t.Fatal("GetRootCAPool returned nil")
	}
	if pool.CertCount() != 1 {
		t.Errorf("expected 1 cert, got %d", pool.CertCount())
	}
}

// TestMTLSManager_LoadClientCert tests loading client certificates.
func TestMTLSManager_LoadClientCert(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CA
	caCertPEM, _, _, caKey, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	// Create client cert signed by CA
	clientCertPEM, clientKeyPEM, _, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, nil, caKey)
	certFile := filepath.Join(tmpDir, "client.crt")
	keyFile := filepath.Join(tmpDir, "client.key")
	os.WriteFile(certFile, clientCertPEM, 0644)
	os.WriteFile(keyFile, clientKeyPEM, 0600)

	manager := NewMTLSManager()
	err := manager.LoadClientCert("test-client", certFile, keyFile)
	if err != nil {
		t.Fatalf("LoadClientCert failed: %v", err)
	}

	cert := manager.GetClientCert("test-client")
	if cert == nil {
		t.Fatal("GetClientCert returned nil")
	}
}

// TestMTLSManager_LoadClientCert_InvalidFile tests loading invalid client certificates.
func TestMTLSManager_LoadClientCert_InvalidFile(t *testing.T) {
	manager := NewMTLSManager()
	err := manager.LoadClientCert("test-client", "/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("expected error for non-existent files")
	}
}

// TestBuildClientTLSConfig tests building client TLS configurations.
func TestBuildClientTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CA
	caCertPEM, _, _, caKey, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	// Create client cert
	clientCertPEM, clientKeyPEM, _, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, nil, caKey)
	certFile := filepath.Join(tmpDir, "client.crt")
	keyFile := filepath.Join(tmpDir, "client.key")
	os.WriteFile(certFile, clientCertPEM, 0644)
	os.WriteFile(keyFile, clientKeyPEM, 0600)

	t.Run("nil config", func(t *testing.T) {
		config, err := BuildClientTLSConfig(nil)
		if err != nil {
			t.Fatalf("BuildClientTLSConfig failed: %v", err)
		}
		if config == nil {
			t.Fatal("expected non-nil config")
		}
		if config.MinVersion != tls.VersionTLS12 {
			t.Error("expected TLS 1.2 minimum")
		}
	})

	t.Run("basic config", func(t *testing.T) {
		mtlsConfig := &MTLSConfig{
			Enabled:            true,
			InsecureSkipVerify: true,
			ServerName:         "example.com",
		}
		config, err := BuildClientTLSConfig(mtlsConfig)
		if err != nil {
			t.Fatalf("BuildClientTLSConfig failed: %v", err)
		}
		if !config.InsecureSkipVerify {
			t.Error("expected InsecureSkipVerify to be true")
		}
		if config.ServerName != "example.com" {
			t.Errorf("expected ServerName example.com, got %s", config.ServerName)
		}
	})

	t.Run("with root CAs", func(t *testing.T) {
		mtlsConfig := &MTLSConfig{
			Enabled: true,
			RootCAs: []string{caFile},
		}
		config, err := BuildClientTLSConfig(mtlsConfig)
		if err != nil {
			t.Fatalf("BuildClientTLSConfig failed: %v", err)
		}
		if config.RootCAs == nil {
			t.Error("expected RootCAs to be set")
		}
	})

	t.Run("with client cert", func(t *testing.T) {
		mtlsConfig := &MTLSConfig{
			Enabled:  true,
			CertFile: certFile,
			KeyFile:  keyFile,
		}
		config, err := BuildClientTLSConfig(mtlsConfig)
		if err != nil {
			t.Fatalf("BuildClientTLSConfig failed: %v", err)
		}
		if len(config.Certificates) != 1 {
			t.Errorf("expected 1 certificate, got %d", len(config.Certificates))
		}
	})

	t.Run("full mTLS config", func(t *testing.T) {
		mtlsConfig := &MTLSConfig{
			Enabled:            true,
			RootCAs:            []string{caFile},
			CertFile:           certFile,
			KeyFile:            keyFile,
			ServerName:         "example.com",
			InsecureSkipVerify: false,
		}
		config, err := BuildClientTLSConfig(mtlsConfig)
		if err != nil {
			t.Fatalf("BuildClientTLSConfig failed: %v", err)
		}
		if config.RootCAs == nil {
			t.Error("expected RootCAs to be set")
		}
		if len(config.Certificates) != 1 {
			t.Errorf("expected 1 certificate, got %d", len(config.Certificates))
		}
		if config.ServerName != "example.com" {
			t.Errorf("expected ServerName example.com, got %s", config.ServerName)
		}
	})
}

// TestBuildClientTLSConfig_InvalidPaths tests error handling for invalid paths.
func TestBuildClientTLSConfig_InvalidPaths(t *testing.T) {
	t.Run("invalid root CA", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled: true,
			RootCAs: []string{"/nonexistent/ca.pem"},
		}
		_, err := BuildClientTLSConfig(config)
		if err == nil {
			t.Error("expected error for invalid root CA path")
		}
	})

	t.Run("invalid cert file", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		}
		_, err := BuildClientTLSConfig(config)
		if err == nil {
			t.Error("expected error for invalid cert file path")
		}
	})
}

// TestMTLSManager_BuildServerTLSConfig tests building server TLS configurations.
func TestMTLSManager_BuildServerTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CA
	caCertPEM, _, _, caKey, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	// Create server cert
	serverCertPEM, serverKeyPEM, _, _, _ := generateTestCertWithCA([]string{"server.example.com"}, false, nil, caKey)

	// Load server certificate
	tlsCert, _ := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	getCert := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return &tlsCert, nil
	}

	manager := NewMTLSManager()

	t.Run("disabled mTLS", func(t *testing.T) {
		config := &MTLSConfig{Enabled: false}
		tlsConfig, err := manager.BuildServerTLSConfig("test", config, getCert)
		if err != nil {
			t.Fatalf("BuildServerTLSConfig failed: %v", err)
		}
		if tlsConfig.ClientAuth != tls.NoClientCert {
			t.Error("expected NoClientCert when disabled")
		}
	})

	t.Run("enabled mTLS", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled:    true,
			ClientAuth: RequireAndVerifyClientCert,
			ClientCAs:  []string{caFile},
		}
		tlsConfig, err := manager.BuildServerTLSConfig("test", config, getCert)
		if err != nil {
			t.Fatalf("BuildServerTLSConfig failed: %v", err)
		}
		if tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Error("expected RequireAndVerifyClientCert")
		}
		if tlsConfig.ClientCAs == nil {
			t.Error("expected ClientCAs to be set")
		}
	})

	t.Run("request client cert", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled:    true,
			ClientAuth: RequestClientCert,
		}
		tlsConfig, err := manager.BuildServerTLSConfig("test", config, getCert)
		if err != nil {
			t.Fatalf("BuildServerTLSConfig failed: %v", err)
		}
		if tlsConfig.ClientAuth != tls.RequestClientCert {
			t.Error("expected RequestClientCert")
		}
	})
}

// TestVerifyClientCert tests client certificate verification.
func TestVerifyClientCert(t *testing.T) {
	// Create CA (self-signed, acting as root)
	_, _, caCert, caKey, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)

	// Create client cert signed by CA
	_, _, clientCert, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, caCert, caKey)

	// Create CA pool with the CA cert as trusted root
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	t.Run("valid certificate", func(t *testing.T) {
		err := VerifyClientCert(clientCert, caPool, 0)
		if err != nil {
			t.Errorf("VerifyClientCert failed: %v", err)
		}
	})

	t.Run("nil certificate", func(t *testing.T) {
		err := VerifyClientCert(nil, caPool, 0)
		if err == nil {
			t.Error("expected error for nil certificate")
		}
	})

	t.Run("wrong CA", func(t *testing.T) {
		// Create different CA
		_, _, otherCACert, otherCAKey, _ := generateTestCertWithCA([]string{"Other CA"}, true, nil, nil)
		_, _, wrongClientCert, _, _ := generateTestCertWithCA([]string{"wrong.example.com"}, false, otherCACert, otherCAKey)

		// Verify with wrong CA pool (should fail)
		err := VerifyClientCert(wrongClientCert, caPool, 0)
		if err == nil {
			t.Error("expected error for certificate signed by different CA")
		}
	})
}

// TestVerifyClientCertWithIntermediates tests verification with intermediate certificates.
func TestVerifyClientCertWithIntermediates(t *testing.T) {
	// Create root CA
	_, _, rootCert, rootKey, _ := generateTestCertWithCA([]string{"Root CA"}, true, nil, nil)

	// Create intermediate CA
	_, _, intermediateCert, intermediateKey, _ := generateTestCertWithCA([]string{"Intermediate CA"}, true, rootCert, rootKey)

	// Create client cert signed by intermediate
	_, _, clientCert, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, intermediateCert, intermediateKey)

	// Create root pool
	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert)

	t.Run("with intermediates", func(t *testing.T) {
		err := VerifyClientCertWithIntermediates(clientCert, []*x509.Certificate{intermediateCert}, rootPool, 0)
		if err != nil {
			t.Errorf("VerifyClientCertWithIntermediates failed: %v", err)
		}
	})

	t.Run("without intermediates", func(t *testing.T) {
		// Should fail without intermediate
		err := VerifyClientCert(clientCert, rootPool, 0)
		if err == nil {
			t.Error("expected error without intermediate certificate")
		}
	})

	t.Run("chain depth limit", func(t *testing.T) {
		// Should succeed with depth 2
		err := VerifyClientCertWithIntermediates(clientCert, []*x509.Certificate{intermediateCert}, rootPool, 2)
		if err != nil {
			t.Errorf("VerifyClientCertWithIntermediates failed: %v", err)
		}

		// Should fail with depth 1 (chain is: client -> intermediate -> root)
		err = VerifyClientCertWithIntermediates(clientCert, []*x509.Certificate{intermediateCert}, rootPool, 1)
		if err == nil {
			t.Error("expected error with insufficient chain depth")
		}
	})
}

// TestGetClientCertInfo tests extracting information from client certificates.
func TestGetClientCertInfo(t *testing.T) {
	t.Run("valid certificate", func(t *testing.T) {
		_, _, cert, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, nil, nil)

		info := GetClientCertInfo(cert)
		if info == nil {
			t.Fatal("expected non-nil info")
		}

		if _, ok := info["subject"]; !ok {
			t.Error("expected subject in info")
		}
		if _, ok := info["issuer"]; !ok {
			t.Error("expected issuer in info")
		}
		if _, ok := info["serial"]; !ok {
			t.Error("expected serial in info")
		}
		if _, ok := info["not_before"]; !ok {
			t.Error("expected not_before in info")
		}
		if _, ok := info["not_after"]; !ok {
			t.Error("expected not_after in info")
		}
	})

	t.Run("nil certificate", func(t *testing.T) {
		info := GetClientCertInfo(nil)
		if info != nil {
			t.Error("expected nil info for nil certificate")
		}
	})

	t.Run("certificate with all fields", func(t *testing.T) {
		// Create a certificate with additional fields
		priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		template := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{"Test Org"},
				CommonName:   "test@example.com",
			},
			DNSNames:       []string{"example.com", "www.example.com"},
			EmailAddresses: []string{"test@example.com"},
			NotBefore:      time.Now(),
			NotAfter:       time.Now().Add(24 * time.Hour),
		}

		certDER, _ := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		cert, _ := x509.ParseCertificate(certDER)

		info := GetClientCertInfo(cert)
		if info == nil {
			t.Fatal("expected non-nil info")
		}

		if dnsNames, ok := info["dns_names"].([]string); !ok || len(dnsNames) != 2 {
			t.Errorf("expected 2 DNS names, got %v", info["dns_names"])
		}

		if emails, ok := info["email_addresses"].([]string); !ok || len(emails) != 1 {
			t.Errorf("expected 1 email address, got %v", info["email_addresses"])
		}
	})
}

// TestDefaultMTLSConfig tests the default mTLS configuration.
func TestDefaultMTLSConfig(t *testing.T) {
	config := DefaultMTLSConfig()

	if config.Enabled {
		t.Error("expected Enabled to be false by default")
	}
	if config.ClientAuth != NoClientCert {
		t.Errorf("expected ClientAuth to be NoClientCert, got %v", config.ClientAuth)
	}
	if config.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be false by default")
	}
	if config.VerifyDepth != 0 {
		t.Errorf("expected VerifyDepth to be 0, got %d", config.VerifyDepth)
	}
	if config.OCSPCheck {
		t.Error("expected OCSPCheck to be false by default")
	}
}

// TestParsePEMCerts tests parsing PEM certificates.
func TestParsePEMCerts(t *testing.T) {
	t.Run("single certificate", func(t *testing.T) {
		certPEM, _, _, _, _ := generateTestCertWithCA([]string{"test.example.com"}, false, nil, nil)

		certs, err := parsePEMCerts(certPEM)
		if err != nil {
			t.Fatalf("parsePEMCerts failed: %v", err)
		}
		if len(certs) != 1 {
			t.Errorf("expected 1 certificate, got %d", len(certs))
		}
	})

	t.Run("multiple certificates", func(t *testing.T) {
		cert1PEM, _, _, _, _ := generateTestCertWithCA([]string{"test1.example.com"}, false, nil, nil)
		cert2PEM, _, _, _, _ := generateTestCertWithCA([]string{"test2.example.com"}, false, nil, nil)

		combined := append(cert1PEM, cert2PEM...)

		certs, err := parsePEMCerts(combined)
		if err != nil {
			t.Fatalf("parsePEMCerts failed: %v", err)
		}
		if len(certs) != 2 {
			t.Errorf("expected 2 certificates, got %d", len(certs))
		}
	})

	t.Run("empty data", func(t *testing.T) {
		_, err := parsePEMCerts([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
	})

	t.Run("invalid PEM", func(t *testing.T) {
		_, err := parsePEMCerts([]byte("not a valid PEM"))
		if err == nil {
			t.Error("expected error for invalid PEM")
		}
	})

	t.Run("non-certificate PEM", func(t *testing.T) {
		// Create a PEM block that's not a certificate
		data := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: []byte("not a real key"),
		})
		_, err := parsePEMCerts(data)
		if err == nil {
			t.Error("expected error for non-certificate PEM")
		}
	})
}

// TestLoadCADirectory tests loading CA certificates from a directory.
func TestLoadCADirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple CA certificates
	ca1PEM, _, _, _, _ := generateTestCertWithCA([]string{"CA 1"}, true, nil, nil)
	ca2PEM, _, _, _, _ := generateTestCertWithCA([]string{"CA 2"}, true, nil, nil)

	// Write certificates with different extensions
	os.WriteFile(filepath.Join(tmpDir, "ca1.pem"), ca1PEM, 0644)
	os.WriteFile(filepath.Join(tmpDir, "ca2.crt"), ca2PEM, 0644)

	// Create a subdirectory (should be skipped)
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	pool := NewCAPool()
	err := loadCADirectory(pool, tmpDir)
	if err != nil {
		t.Fatalf("loadCADirectory failed: %v", err)
	}

	if pool.CertCount() != 2 {
		t.Errorf("expected 2 certificates, got %d", pool.CertCount())
	}
}

// TestLoadCADirectory_Empty tests loading from an empty directory.
func TestLoadCADirectory_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	pool := NewCAPool()
	err := loadCADirectory(pool, tmpDir)
	if err != nil {
		t.Fatalf("loadCADirectory failed: %v", err)
	}

	if pool.CertCount() != 0 {
		t.Errorf("expected 0 certificates, got %d", pool.CertCount())
	}
}

// TestLoadCAFile_InvalidFile tests loading an invalid CA file.
func TestLoadCAFile_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.pem")
	os.WriteFile(invalidFile, []byte("not a valid certificate"), 0644)

	pool := NewCAPool()
	err := loadCAFile(pool, invalidFile)
	if err == nil {
		t.Error("expected error for invalid certificate file")
	}
}

// TestMTLSManager_ConcurrentAccess tests concurrent access to the manager.
func TestMTLSManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test certificates
	caCertPEM, _, _, caKey, _ := generateTestCertWithCA([]string{"Test CA"}, true, nil, nil)
	caFile := filepath.Join(tmpDir, "ca.pem")
	os.WriteFile(caFile, caCertPEM, 0644)

	clientCertPEM, clientKeyPEM, _, _, _ := generateTestCertWithCA([]string{"client.example.com"}, false, nil, caKey)
	certFile := filepath.Join(tmpDir, "client.crt")
	keyFile := filepath.Join(tmpDir, "client.key")
	os.WriteFile(certFile, clientCertPEM, 0644)
	os.WriteFile(keyFile, clientKeyPEM, 0600)

	manager := NewMTLSManager()

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			name := fmt.Sprintf("pool-%d", idx)
			manager.LoadClientCAPool(name, []string{caFile})
			manager.LoadRootCAPool(name, []string{caFile})
			manager.LoadClientCert(name, certFile, keyFile)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(idx int) {
			name := fmt.Sprintf("pool-%d", idx)
			_ = manager.GetClientCAPool(name)
			_ = manager.GetRootCAPool(name)
			_ = manager.GetClientCert(name)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestCAPool_ConcurrentAccess tests concurrent access to the CA pool.
func TestCAPool_ConcurrentAccess(t *testing.T) {
	pool := NewCAPool()

	// Create test certificates
	_, _, ca1, _, _ := generateTestCertWithCA([]string{"CA 1"}, true, nil, nil)
	_, _, ca2, _, _ := generateTestCertWithCA([]string{"CA 2"}, true, nil, nil)

	done := make(chan bool, 20)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(idx int) {
			if idx%2 == 0 {
				pool.AddCert(ca1)
			} else {
				pool.AddCert(ca2)
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_ = pool.CertCount()
			_ = pool.Subjects()
			_ = pool.Pool()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	if pool.CertCount() != 10 {
		t.Errorf("expected 10 certificates, got %d", pool.CertCount())
	}
}

// TestMTLSConfig_Validate_CRLFile tests CRL file validation.
func TestMTLSConfig_Validate_CRLFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid CRL file
	crlFile := filepath.Join(tmpDir, "crl.pem")
	crlData := []byte("-----BEGIN X509 CRL-----\ntest\n-----END X509 CRL-----")
	os.WriteFile(crlFile, crlData, 0644)

	t.Run("valid CRL file", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled: true,
			CRLFile: crlFile,
		}
		err := config.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("non-existent CRL file", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled: true,
			CRLFile: "/nonexistent/crl.pem",
		}
		err := config.Validate()
		if err == nil {
			t.Error("expected error for non-existent CRL file")
		}
	})
}

// TestMTLSManager_BuildServerTLSConfig_InvalidConfig tests error handling.
func TestMTLSManager_BuildServerTLSConfig_InvalidConfig(t *testing.T) {
	manager := NewMTLSManager()

	// Create a simple getCertificate function
	getCert := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return nil, nil
	}

	t.Run("invalid client CAs", func(t *testing.T) {
		config := &MTLSConfig{
			Enabled:    true,
			ClientAuth: RequireAndVerifyClientCert,
			ClientCAs:  []string{"/nonexistent/ca.pem"},
		}
		_, err := manager.BuildServerTLSConfig("test", config, getCert)
		if err == nil {
			t.Error("expected error for invalid client CAs")
		}
	})
}
