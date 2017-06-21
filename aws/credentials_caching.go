/*
Copyright 2017 WALLIX

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

package aws

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/wallix/awless/logger"
)

type cachedCredential struct {
	credentials.Value
	Expiration time.Time
}

func (c *cachedCredential) isExpired() bool {
	return c.Expiration.Before(time.Now().UTC())
}

type fileCacheProvider struct {
	creds   *credentials.Credentials
	curr    *cachedCredential
	profile string
	log     *logger.Logger
}

func (f *fileCacheProvider) Retrieve() (credentials.Value, error) {
	awlessCache := os.Getenv("__AWLESS_CACHE")
	if awlessCache == "" {
		return f.creds.Get()
	}
	credFolder := filepath.Join(awlessCache, "credentials")
	if _, err := os.Stat(credFolder); os.IsNotExist(err) {
		os.MkdirAll(credFolder, 0700)
	}
	credFile := fmt.Sprintf("aws-profile-%s.json", f.profile)
	credPath := filepath.Join(credFolder, credFile)

	if _, readerr := os.Stat(credPath); readerr == nil {
		var cached *cachedCredential
		content, err := ioutil.ReadFile(credPath)
		if err != nil {
			return cached.Value, err
		}
		err = json.Unmarshal(content, &cached)
		f.log.ExtraVerbosef("loading credentials from '%s'", credPath)
		if !cached.isExpired() {
			f.curr = cached
			return cached.Value, nil
		}
	}
	f.log.ExtraVerbose("no valid cached credentials, getting new credentials")
	credValue, err := f.creds.Get()
	if err != nil {
		return credValue, err
	}

	switch credValue.ProviderName {
	case "AssumeRoleProvider":
		cred := &cachedCredential{credValue, time.Now().UTC().Add(stscreds.DefaultDuration)}
		f.curr = cred
		content, err := json.Marshal(cred)
		if err != nil {
			return credValue, err
		}
		ioutil.WriteFile(credPath, content, 0600)
		f.log.ExtraVerbosef("credentials cached in '%s'", credPath)
		return credValue, nil
	}
	return credValue, nil
}
func (f *fileCacheProvider) IsExpired() bool {
	if f.curr != nil {
		return f.curr.isExpired()
	}
	return f.creds.IsExpired()
}
