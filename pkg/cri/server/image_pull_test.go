/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package server

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/containerd/containerd/pkg/cri/annotations"
	criconfig "github.com/containerd/containerd/pkg/cri/config"
	"github.com/containerd/containerd/pkg/cri/labels"

	"github.com/stretchr/testify/assert"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func TestParseAuth(t *testing.T) {
	testUser := "username"
	testPasswd := "password"
	testAuthLen := base64.StdEncoding.EncodedLen(len(testUser + ":" + testPasswd))
	testAuth := make([]byte, testAuthLen)
	base64.StdEncoding.Encode(testAuth, []byte(testUser+":"+testPasswd))
	invalidAuth := make([]byte, testAuthLen)
	base64.StdEncoding.Encode(invalidAuth, []byte(testUser+"@"+testPasswd))
	for _, test := range []struct {
		desc           string
		auth           *runtime.AuthConfig
		host           string
		expectedUser   string
		expectedSecret string
		expectErr      bool
	}{
		{
			desc: "should not return error if auth config is nil",
		},
		{
			desc:      "should not return error if empty auth is provided for access to anonymous registry",
			auth:      &runtime.AuthConfig{},
			expectErr: false,
		},
		{
			desc:           "should support identity token",
			auth:           &runtime.AuthConfig{IdentityToken: "abcd"},
			expectedSecret: "abcd",
		},
		{
			desc: "should support username and password",
			auth: &runtime.AuthConfig{
				Username: testUser,
				Password: testPasswd,
			},
			expectedUser:   testUser,
			expectedSecret: testPasswd,
		},
		{
			desc:           "should support auth",
			auth:           &runtime.AuthConfig{Auth: string(testAuth)},
			expectedUser:   testUser,
			expectedSecret: testPasswd,
		},
		{
			desc:      "should return error for invalid auth",
			auth:      &runtime.AuthConfig{Auth: string(invalidAuth)},
			expectErr: true,
		},
		{
			desc: "should return empty auth if server address doesn't match",
			auth: &runtime.AuthConfig{
				Username:      testUser,
				Password:      testPasswd,
				ServerAddress: "https://registry-1.io",
			},
			host:           "registry-2.io",
			expectedUser:   "",
			expectedSecret: "",
		},
		{
			desc: "should return auth if server address matches",
			auth: &runtime.AuthConfig{
				Username:      testUser,
				Password:      testPasswd,
				ServerAddress: "https://registry-1.io",
			},
			host:           "registry-1.io",
			expectedUser:   testUser,
			expectedSecret: testPasswd,
		},
		{
			desc: "should return auth if server address is not specified",
			auth: &runtime.AuthConfig{
				Username: testUser,
				Password: testPasswd,
			},
			host:           "registry-1.io",
			expectedUser:   testUser,
			expectedSecret: testPasswd,
		},
	} {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			u, s, err := ParseAuth(test.auth, test.host)
			assert.Equal(t, test.expectErr, err != nil)
			assert.Equal(t, test.expectedUser, u)
			assert.Equal(t, test.expectedSecret, s)
		})
	}
}

func TestRegistryEndpoints(t *testing.T) {
	for _, test := range []struct {
		desc     string
		mirrors  map[string]criconfig.Mirror
		host     string
		expected []string
	}{
		{
			desc: "no mirror configured",
			mirrors: map[string]criconfig.Mirror{
				"registry-1.io": {
					Endpoints: []string{
						"https://registry-1.io",
						"https://registry-2.io",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-3.io",
			},
		},
		{
			desc: "mirror configured",
			mirrors: map[string]criconfig.Mirror{
				"registry-3.io": {
					Endpoints: []string{
						"https://registry-1.io",
						"https://registry-2.io",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-1.io",
				"https://registry-2.io",
				"https://registry-3.io",
			},
		},
		{
			desc: "wildcard mirror configured",
			mirrors: map[string]criconfig.Mirror{
				"*": {
					Endpoints: []string{
						"https://registry-1.io",
						"https://registry-2.io",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-1.io",
				"https://registry-2.io",
				"https://registry-3.io",
			},
		},
		{
			desc: "host should take precedence if both host and wildcard mirrors are configured",
			mirrors: map[string]criconfig.Mirror{
				"*": {
					Endpoints: []string{
						"https://registry-1.io",
					},
				},
				"registry-3.io": {
					Endpoints: []string{
						"https://registry-2.io",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-2.io",
				"https://registry-3.io",
			},
		},
		{
			desc: "default endpoint in list with http",
			mirrors: map[string]criconfig.Mirror{
				"registry-3.io": {
					Endpoints: []string{
						"https://registry-1.io",
						"https://registry-2.io",
						"http://registry-3.io",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-1.io",
				"https://registry-2.io",
				"http://registry-3.io",
			},
		},
		{
			desc: "default endpoint in list with https",
			mirrors: map[string]criconfig.Mirror{
				"registry-3.io": {
					Endpoints: []string{
						"https://registry-1.io",
						"https://registry-2.io",
						"https://registry-3.io",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-1.io",
				"https://registry-2.io",
				"https://registry-3.io",
			},
		},
		{
			desc: "default endpoint in list with path",
			mirrors: map[string]criconfig.Mirror{
				"registry-3.io": {
					Endpoints: []string{
						"https://registry-1.io",
						"https://registry-2.io",
						"https://registry-3.io/path",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-1.io",
				"https://registry-2.io",
				"https://registry-3.io/path",
			},
		},
		{
			desc: "miss scheme endpoint in list with path",
			mirrors: map[string]criconfig.Mirror{
				"registry-3.io": {
					Endpoints: []string{
						"https://registry-3.io",
						"registry-1.io",
						"127.0.0.1:1234",
					},
				},
			},
			host: "registry-3.io",
			expected: []string{
				"https://registry-3.io",
				"https://registry-1.io",
				"http://127.0.0.1:1234",
			},
		},
	} {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			c := newTestCRIService()
			c.config.Registry.Mirrors = test.mirrors
			got, err := c.registryEndpoints(test.host)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, got)
		})
	}
}

func TestDefaultScheme(t *testing.T) {
	for _, test := range []struct {
		desc     string
		host     string
		expected string
	}{
		{
			desc:     "should use http by default for localhost",
			host:     "localhost",
			expected: "http",
		},
		{
			desc:     "should use http by default for localhost with port",
			host:     "localhost:8080",
			expected: "http",
		},
		{
			desc:     "should use http by default for 127.0.0.1",
			host:     "127.0.0.1",
			expected: "http",
		},
		{
			desc:     "should use http by default for 127.0.0.1 with port",
			host:     "127.0.0.1:8080",
			expected: "http",
		},
		{
			desc:     "should use http by default for ::1",
			host:     "::1",
			expected: "http",
		},
		{
			desc:     "should use http by default for ::1 with port",
			host:     "[::1]:8080",
			expected: "http",
		},
		{
			desc:     "should use https by default for remote host",
			host:     "remote",
			expected: "https",
		},
		{
			desc:     "should use https by default for remote host with port",
			host:     "remote:8080",
			expected: "https",
		},
		{
			desc:     "should use https by default for remote ip",
			host:     "8.8.8.8",
			expected: "https",
		},
		{
			desc:     "should use https by default for remote ip with port",
			host:     "8.8.8.8:8080",
			expected: "https",
		},
	} {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			got := defaultScheme(test.host)
			assert.Equal(t, test.expected, got)
		})
	}
}

func TestEncryptedImagePullOpts(t *testing.T) {
	for _, test := range []struct {
		desc         string
		keyModel     string
		expectedOpts int
	}{
		{
			desc:         "node key model should return one unpack opt",
			keyModel:     criconfig.KeyModelNode,
			expectedOpts: 1,
		},
		{
			desc:         "no key model selected should default to node key model",
			keyModel:     "",
			expectedOpts: 0,
		},
	} {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			c := newTestCRIService()
			c.config.ImageDecryption.KeyModel = test.keyModel
			got := len(c.encryptedImagesPullOpts())
			assert.Equal(t, test.expectedOpts, got)
		})
	}
}

func TestSnapshotterFromPodSandboxConfig(t *testing.T) {
	defaultSnashotter := "native"
	runtimeSnapshotter := "devmapper"
	tests := []struct {
		desc              string
		podSandboxConfig  *runtime.PodSandboxConfig
		expectSnapshotter string
		expectErr         bool
	}{
		{
			desc:              "should return default snapshotter for nil podSandboxConfig",
			expectSnapshotter: defaultSnashotter,
		},
		{
			desc:              "should return default snapshotter for nil podSandboxConfig.Annotations",
			podSandboxConfig:  &runtime.PodSandboxConfig{},
			expectSnapshotter: defaultSnashotter,
		},
		{
			desc: "should return default snapshotter for empty podSandboxConfig.Annotations",
			podSandboxConfig: &runtime.PodSandboxConfig{
				Annotations: make(map[string]string),
			},
			expectSnapshotter: defaultSnashotter,
		},
		{
			desc: "should return error for runtime not found",
			podSandboxConfig: &runtime.PodSandboxConfig{
				Annotations: map[string]string{
					annotations.RuntimeHandler: "runtime-not-exists",
				},
			},
			expectErr:         true,
			expectSnapshotter: "",
		},
		{
			desc: "should return snapshotter provided in podSandboxConfig.Annotations",
			podSandboxConfig: &runtime.PodSandboxConfig{
				Annotations: map[string]string{
					annotations.RuntimeHandler: "exiting-runtime",
				},
			},
			expectSnapshotter: runtimeSnapshotter,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			cri := newTestCRIService()
			cri.config.ContainerdConfig.Snapshotter = defaultSnashotter
			cri.config.ContainerdConfig.Runtimes = make(map[string]criconfig.Runtime)
			cri.config.ContainerdConfig.Runtimes["exiting-runtime"] = criconfig.Runtime{
				Snapshotter: runtimeSnapshotter,
			}
			snapshotter, err := cri.snapshotterFromPodSandboxConfig(context.Background(), "test-image", tt.podSandboxConfig)
			assert.Equal(t, tt.expectSnapshotter, snapshotter)
			if tt.expectErr {
				assert.Error(t, err)
			}
		})
	}
}
func TestImageGetLabels(t *testing.T) {
	tests := []struct {
		name               string
		expectedLabel      map[string]string
		configSandboxImage string
		pullImageName      string
	}{
		{
			name:               "pinned image labels should get added on sandbox image",
			expectedLabel:      map[string]string{labels.ImageLabelKey: labels.ImageLabelValue, labels.PinnedImageLabelKey: labels.PinnedImageLabelValue},
			configSandboxImage: "registry.k8s.io/pause:3.9",
			pullImageName:      "registry.k8s.io/pause:3.9",
		},
		{
			name:               "pinned image labels should get added on sandbox image without tag",
			expectedLabel:      map[string]string{labels.ImageLabelKey: labels.ImageLabelValue, labels.PinnedImageLabelKey: labels.PinnedImageLabelValue},
			configSandboxImage: "registry.k8s.io/pause",
			pullImageName:      "registry.k8s.io/pause:latest",
		},
		{
			name:               "pinned image labels should get added on sandbox image specified with tag and digest both",
			expectedLabel:      map[string]string{labels.ImageLabelKey: labels.ImageLabelValue, labels.PinnedImageLabelKey: labels.PinnedImageLabelValue},
			configSandboxImage: "registry.k8s.io/pause:3.9@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",
			pullImageName:      "registry.k8s.io/pause@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",
		},
		{
			name:               "pinned image labels should get added on sandbox image specified with digest",
			expectedLabel:      map[string]string{labels.ImageLabelKey: labels.ImageLabelValue, labels.PinnedImageLabelKey: labels.PinnedImageLabelValue},
			configSandboxImage: "registry.k8s.io/pause@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",
			pullImageName:      "registry.k8s.io/pause@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",
		},
		{
			name:               "pinned image labels should not get added on other image",
			expectedLabel:      map[string]string{labels.ImageLabelKey: labels.ImageLabelValue},
			configSandboxImage: "registry.k8s.io/pause:3.9",
			pullImageName:      "registry.k8s.io/random:latest",
		},
	}

	svc := newTestCRIService()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc.config.SandboxImage = tc.configSandboxImage
			assert.Equal(t, tc.expectedLabel, svc.getLabels(context.Background(), tc.pullImageName))
		})
	}
}
