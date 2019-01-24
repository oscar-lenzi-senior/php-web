/*
 * Copyright 2018-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package phpweb

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/libcfbuildpack/helper"

	bplogger "github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/cloudfoundry/libcfbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/test"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitContributor(t *testing.T) {
	spec.Run(t, "Contributor", testContributor, spec.Report(report.Terminal{}))
}

func testContributor(t *testing.T, when spec.G, it spec.S) {
	var f *test.BuildFactory
	var c Contributor

	it.Before(func() {
		RegisterTestingT(t)
		var err error
		f = test.NewBuildFactory(t)
		c, _, err = NewContributor(f.Build)
		Expect(err).To(Not(HaveOccurred()))
	})

	it("starts a web app with `php -S`", func() {
		c.isWebApp = true
		c.webserver = PhpWebServer

		Expect(c.Contribute()).To(Succeed())

		command := fmt.Sprintf("php -S 0.0.0.0:8080 -t %s/%s", f.Build.Application.Root, "htdocs")
		Expect(f.Build.Layers).To(test.HaveLaunchMetadata(layers.Metadata{
			Processes: []layers.Process{
				{"web", command},
				{"task", command},
			},
		}))
	})

	it("starts a web app with a custom webdir", func() {
		c.isWebApp = true
		c.webserver = PhpWebServer
		c.webdir = "public"

		Expect(c.Contribute()).To(Succeed())

		command := fmt.Sprintf("php -S 0.0.0.0:8080 -t %s/%s", f.Build.Application.Root, "public")
		Expect(f.Build.Layers).To(test.HaveLaunchMetadata(layers.Metadata{
			Processes: []layers.Process{
				{"web", command},
				{"task", command},
			},
		}))
	})

	it("contributes a php.ini file & configures PHP to look at it for a web app", func() {
		c.isWebApp = true
		c.webserver = PhpWebServer

		layer := f.Build.Layers.Layer(WebDependency)
		Expect(c.Contribute()).To(Succeed())
		Expect(filepath.Join(layer.Root, "etc", "php.ini")).To(BeARegularFile())
		Expect(layer).To(test.HaveOverrideSharedEnvironment("PHPRC", filepath.Join(layer.Root, "etc")))
		Expect(layer).To(test.HaveOverrideSharedEnvironment("PHP_INI_SCAN_DIR", filepath.Join(f.Build.Application.Root, ".php.ini.d")))
	})

	it("contributes a php.ini file & configures PHP to look at it for a script", func() {
		c.isScript = true

		layer := f.Build.Layers.Layer(ScriptDependency)
		Expect(c.Contribute()).To(Succeed())
		Expect(filepath.Join(layer.Root, "etc", "php.ini")).To(BeARegularFile())
		Expect(layer).To(test.HaveOverrideSharedEnvironment("PHPRC", filepath.Join(layer.Root, "etc")))
		Expect(layer).To(test.HaveOverrideSharedEnvironment("PHP_INI_SCAN_DIR", filepath.Join(f.Build.Application.Root, ".php.ini.d")))
	})

	it("starts a web app with HTTPD", func() {
		c.isWebApp = true
		c.webserver = ApacheHttpd

		Expect(c.Contribute()).To(Succeed())

		phpLayer := f.Build.Layers.Layer(WebDependency)

		command := fmt.Sprintf(`php-fpm -p "%s" -y "%s" -c "%s"`,
			phpLayer.Root,
			filepath.Join(phpLayer.Root, "etc", "php-fpm.conf"),
			filepath.Join(phpLayer.Root, "etc"))
		Expect(f.Build.Layers).To(test.HaveLaunchMetadata(layers.Metadata{
			Processes: []layers.Process{
				{"web", command},
			},
		}))
	})

	it("starts a web app and defaults to Apache Web Server", func() {
		c.isWebApp = true

		Expect(c.Contribute()).To(Succeed())

		phpLayer := f.Build.Layers.Layer(WebDependency)

		command := fmt.Sprintf(`php-fpm -p "%s" -y "%s" -c "%s"`,
			phpLayer.Root,
			filepath.Join(phpLayer.Root, "etc", "php-fpm.conf"),
			filepath.Join(phpLayer.Root, "etc"))
		Expect(f.Build.Layers).To(test.HaveLaunchMetadata(layers.Metadata{
			Processes: []layers.Process{
				{"web", command},
			},
		}))
	})

	it("contributes a httpd.conf & php-fpm.conf file when using Apache Web Server", func() {
		c.isWebApp = true
		c.webserver = ApacheHttpd

		layer := f.Build.Layers.Layer(WebDependency)
		Expect(c.Contribute()).To(Succeed())
		Expect(filepath.Join(f.Build.Application.Root, "httpd.conf")).To(BeARegularFile())
		Expect(filepath.Join(layer.Root, "etc", "php-fpm.conf")).To(BeARegularFile())
	})

	it("contributes php-fpm.conf & includes a user's config", func() {
		helper.WriteFile(filepath.Join(f.Build.Application.Root, ".php.fpm.d", "user.conf"), 0644, "")

		layer := f.Build.Layers.Layer(WebDependency)
		err := c.writePhpFpmConf(layer)

		Expect(err).ToNot(HaveOccurred())
		Expect(filepath.Join(layer.Root, "etc", "php-fpm.conf")).To(BeARegularFile())

		result, err := ioutil.ReadFile(filepath.Join(layer.Root, "etc", "php-fpm.conf"))

		Expect(err).ToNot(HaveOccurred())
		Expect(string(result)).To(ContainSubstring(fmt.Sprintf(`include=%s`, filepath.Join(f.Build.Application.Root, ".php.fpm.d", "*.conf"))))
	})

	it("starts a web app with Nginx", func() {
		// TODO
	})

	it("starts a script using default `app.php`", func() {
		c.isScript = true

		Expect(c.Contribute()).To(Succeed())

		command := fmt.Sprintf("php %s/%s", f.Build.Application.Root, "app.php")
		Expect(f.Build.Layers).To(test.HaveLaunchMetadata(layers.Metadata{
			Processes: []layers.Process{
				{"web", command},
				{"task", command},
			},
		}))
	})

	it("starts a script using custom script path/name", func() {
		c.isScript = true
		c.script = "relative/path/to/my/script.php"

		Expect(c.Contribute()).To(Succeed())

		command := fmt.Sprintf("php %s/%s", f.Build.Application.Root, "relative/path/to/my/script.php")
		Expect(f.Build.Layers).To(test.HaveLaunchMetadata(layers.Metadata{
			Processes: []layers.Process{
				{"web", command},
				{"task", command},
			},
		}))
	})

	it("logs a warning when start script does not exist", func() {
		debug := &bytes.Buffer{}
		info := &bytes.Buffer{}

		c.logger = logger.Logger{Logger: bplogger.NewLogger(debug, info)}
		c.isScript = true
		c.script = "does/not/exist.php"

		Expect(c.Contribute()).To(Succeed())
		Expect(info.String()).To(ContainSubstring("WARNING: `does/not/exist.php` start script not found. App will not start unless you specify a custom start command."))
	})
}
