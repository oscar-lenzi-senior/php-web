package integration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/dagger"
	"github.com/BurntSushi/toml"

	. "github.com/onsi/gomega"
)

var (
	phpDistURI              string
	phpDistOfflineURI       string
	httpdURI					      string
	httpdOfflineURI					string
	nginxURI					      string
	nginxOfflineURI					string
	phpWebURI					      string
	phpWebOfflineURI	      string
	buildpackInfo           struct {
		Buildpack struct {
			ID   string
			Name string
		}
	}
)

// PreparePhpBps builds the current buildpacks
func PreparePhpBps() error {
	bpRoot, err := filepath.Abs("./..")
	Expect(err).ToNot(HaveOccurred())

	file, err := os.Open("../buildpack.toml")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	_, err = toml.DecodeReader(file, &buildpackInfo)
	Expect(err).NotTo(HaveOccurred())

	// Later todo: These buildpack urls redirect from the old cf cnb urls.
	// When rewriting with packit, change them.
	phpDistURI, err = dagger.GetLatestBuildpack("php-dist-cnb")
	if err != nil {
		return err
	}

	phpDistRepo, err := dagger.GetLatestUnpackagedBuildpack("php-dist-cnb")
	Expect(err).ToNot(HaveOccurred())

	phpDistOfflineURI, _, err = dagger.PackageCachedBuildpack(phpDistRepo)
	Expect(err).ToNot(HaveOccurred())

	httpdURI, err = dagger.GetLatestBuildpack("httpd-cnb")
	if err != nil {
		return err
	}

	nginxURI, err = dagger.GetLatestBuildpack("nginx-cnb")
	if err != nil {
		return err
	}

	nginxRepo, err := dagger.GetLatestUnpackagedBuildpack("nginx-cnb")
	Expect(err).ToNot(HaveOccurred())

	nginxOfflineURI, _, err = dagger.PackageCachedBuildpack(nginxRepo)
	Expect(err).ToNot(HaveOccurred())

	nginxOfflineURI = fmt.Sprintf("%s.tgz", nginxOfflineURI)

	httpdRepo, err := dagger.GetLatestUnpackagedBuildpack("httpd-cnb")
	Expect(err).ToNot(HaveOccurred())

	httpdOfflineURI, _, err = dagger.PackageCachedBuildpack(httpdRepo)
	Expect(err).ToNot(HaveOccurred())

	httpdOfflineURI = fmt.Sprintf("%s.tgz", httpdOfflineURI)

	phpWebURI, err = dagger.PackageBuildpack(bpRoot)
	if err != nil {
		return err
	}


	phpWebOfflineURI, _, err = dagger.PackageCachedBuildpack(bpRoot)
	if err != nil {
		return err
	}

	return nil
}

// CleanUpBps removes the packaged buildpacks
func CleanUpBps() {
	for _, bp := range []string{phpDistURI, phpDistOfflineURI, httpdURI, httpdOfflineURI, nginxURI, nginxOfflineURI, phpWebURI, phpWebOfflineURI} {
		Expect(dagger.DeleteBuildpack(bp)).To(Succeed())
	}
}

func PreparePhpApp(appName string, buildpacks []string, env map[string]string) (*dagger.App, error) {
	app, err := dagger.NewPack(
		filepath.Join("testdata", appName),
		dagger.RandomImage(),
		dagger.SetEnv(env),
		dagger.SetBuildpacks(buildpacks...),
		dagger.SetVerbose(),
	).Build()
	if err != nil {
		return nil, err
	}

	app.SetHealthCheck("", "3s", "1s")
	if env == nil {
		env = make(map[string]string)
	}
	env["PORT"] = "8080"
	app.Env = env

	return app, nil
}

func PushSimpleApp(name string, buildpacks []string, script bool) (*dagger.App, error) {
	app, err := PreparePhpApp(name, buildpacks, nil)
	if err != nil {
		return app, err
	}

	if script {
		app.SetHealthCheck("true", "3s", "1s")
	}

	err = app.Start()
	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
		if err != nil {
			return app, err
		}

		containerID, imageName, volumeIDs, err := app.Info()
		if err != nil {
			return app, err
		}

		fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

		containerLogs, err := app.Logs()
		if err != nil {
			return app, err
		}

		fmt.Printf("Container Logs:\n %s\n", containerLogs)
		return app, err
	}

	return app, nil
}
