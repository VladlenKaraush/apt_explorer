package api

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Package: xserver-xorg-video-fbdev-udeb
// Architecture: amd64
// Version: 1:0.5.0-1ubuntu1
// Priority: optional
// Section: universe/debian-installer
// Source: xserver-xorg-video-fbdev
// Maintainer: Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>
// Original-Maintainer: Debian X Strike Force <debian-x@lists.debian.org>
// Installed-Size: 34
// Provides: xorg-driver-video
// Depends: libc6-udeb (>= 2.28), xorg-video-abi-24, xserver-xorg-core-udeb (>= 2:1.18.99.901)
// Filename: pool/universe/x/xserver-xorg-video-fbdev/xserver-xorg-video-fbdev-udeb_0.5.0-1ubuntu1_amd64.udeb
// Size: 8456
// MD5sum: 7a8a44b5c6251a87e48d466466efd7d9
// SHA1: 3a6835745f456c84d8ce553c07bfc4cf94020b45
// SHA256: 0ab3da70a432107abe8c81b0e003a753c6ae9e298227a6d585f35417efd5de63
// Description: X.Org X server -- fbdev display driver
//  This is a udeb, or a microdeb, for the debian-installer.

type ReleaseIndex struct {
	Id, ReleaseUrl, Registry, Suite string
}

type Package struct {
	Registry, Suite, Name, Arch, Version, Depends, Provides, Filename, Size, Md5, Sha1, Sha256, Desc string
}

type ReleaseDesc struct {
	contents []byte
	baseUrl  string
}

type PackageIndDesc struct {
	contents                 []byte
	release, arch, component string
}

func parsePackage(pkgBlock, registry, suite string) Package {
	pkg := Package{
		Registry: registry,
		Suite:    suite,
	}
	pkgLines := strings.Split(pkgBlock, "\n")
	for _, line := range pkgLines {
		splits := strings.Split(line, ":")
		if len(splits) < 2 {
			continue
		}
		name, value := splits[0], splits[1]
		switch name {
		case "Package":
			pkg.Name = value
		case "Architecture":
			pkg.Arch = value
		case "Version":
			pkg.Version = value
		case "Depends":
			pkg.Depends = value
		case "Provides":
			pkg.Provides = value
		case "Filename":
			pkg.Filename = value
		case "Size":
			pkg.Size = value
		case "MD5Sum":
			pkg.Md5 = value
		case "SHA1":
			pkg.Sha1 = value
		case "SHA256":
			pkg.Sha256 = value
		case "Description":
			pkg.Desc = value
		}
	}
	return pkg
}

func parsePackageIndex(pkgSplits []string, registry, suite string, pkgChan chan<- Package, wg *sync.WaitGroup) {
	for _, pkgBlock := range pkgSplits {
		pkg := parsePackage(pkgBlock, registry, suite)
		if len(pkg.Name) > 0 {
			pkgChan <- pkg
		}
		wg.Done()
	}
}

func CollectRelease(url string) []byte {
	resp, _ := http.Get(url)
	index, _ := io.ReadAll(resp.Body)
	return index
}

func CollectReleaseChan(url string, c chan<- ReleaseDesc) {
	basePath := strings.Replace(url, "Release", "", 1)
	resp, _ := http.Get(url)
	index, _ := io.ReadAll(resp.Body)
	fmt.Println("release is read, passing to channel", url)
	c <- ReleaseDesc{index, basePath}
	fmt.Println("release is passed, closing routine", url)
}

func getAndUnzipPkgIndex(basePath, hashLine string) []byte {
	split := strings.Split(hashLine, " ")
	pkgIndPath := basePath + split[len(split)-1]
	fmt.Println("pkg ind path: ", pkgIndPath)
	pkgIndResp, err := http.Get(pkgIndPath)
	if err != nil {
		panic(err)
	}
	gzReader, err := gzip.NewReader(pkgIndResp.Body)
	if err != nil {
		panic(err)
	}
	pkgIndex, err := io.ReadAll(gzReader)
	if err != nil {
		panic(err)
	}
	return pkgIndex
}

func GetPkgIndices(basePath, releaseIndex string) chan []byte {
	lines := strings.Split(releaseIndex, "\n")
	hashLines := []string{}
	foundSha256 := false
	for _, line := range lines {
		if strings.HasPrefix(line, "SHA256:") {
			foundSha256 = true
		}
		if foundSha256 && strings.Contains(line, "binary-amd64") && strings.HasSuffix(line, "Packages.gz") {
			fmt.Println("hash line: ", line)
			hashLines = append(hashLines, line)
		}
	}

	var wg sync.WaitGroup

	pkgInds := make(chan []byte)
	for _, line := range hashLines {
		wg.Add(1)
		go func(pkgChan chan []byte) {
			defer wg.Done()
			pkgIndex := getAndUnzipPkgIndex(basePath, line)
			pkgChan <- pkgIndex
		}(pkgInds)
	}
	go func() {
		wg.Wait()
		close(pkgInds)
	}()
	return pkgInds
}

func GetReleases(releases []ReleaseIndex) chan ReleaseDesc {
	releaseChan := make(chan ReleaseDesc)
	var wg sync.WaitGroup
	for _, release := range releases {
		wg.Add(1)
		fmt.Println("parsing release: ", release.ReleaseUrl)
		go func(url string, c chan<- ReleaseDesc, wg *sync.WaitGroup) {
			CollectReleaseChan(url, c)
			defer wg.Done()
		}(release.ReleaseUrl, releaseChan, &wg)
	}
	go func() {
		fmt.Println("closing on release reading channel")
		wg.Wait()
		close(releaseChan)
	}()
	return releaseChan
}

func GetPackageIndices(releaseCh <-chan ReleaseDesc) <-chan PackageIndDesc {
	pkgIndexCh := make(chan PackageIndDesc)
	var wg sync.WaitGroup
	go func() {
		for release := range releaseCh {
			lines := strings.Split(string(release.contents), "\n")
			hashLines := []string{}
			foundSha256 := false
			for _, line := range lines {
				if strings.HasPrefix(line, "SHA256:") {
					foundSha256 = true
				}
				if foundSha256 && strings.Contains(line, "binary-amd64") && strings.HasSuffix(line, "Packages.gz") {
					hashLines = append(hashLines, line)
				}
			}

			for _, line := range hashLines {
				wg.Add(1)
				go func(pkgChan chan<- PackageIndDesc, release ReleaseDesc) {
					pkgIndex := getAndUnzipPkgIndex(release.baseUrl, line)
					pkgChan <- PackageIndDesc{pkgIndex, "release", "arch", "component"}
					wg.Done()
				}(pkgIndexCh, release)
			}
		}
		go func() {
			wg.Wait()
			close(pkgIndexCh)
		}()
	}()

	return pkgIndexCh
}

func CollectPkgIndicesForRelease(url string) chan []byte {
	release := string(CollectRelease(url))
	basePath := strings.Replace(url, "Release", "", 1)
	return GetPkgIndices(basePath, release)
}

func ApiReleases(releasePaths []string) []byte {

	res := []byte{}
	for _, release := range releasePaths {
		res = append(res, CollectRelease(release)...)
	}
	return res
}

func ApiAvailableReleases(releasePaths []string) []byte {
	res := []byte{}
	for i, release := range releasePaths {
		line := fmt.Sprintf(strconv.Itoa(i) + ": " + release + "\n")
		res = append(res, []byte(line)...)
	}
	return res
}

func GetPkgs(pkgIndCh <-chan PackageIndDesc) {

}

func ApiGetPackages(releases []ReleaseIndex) chan Package {
	pkgChan := make(chan Package)
	var wg sync.WaitGroup

	releasesCh := GetReleases(releases)
	pkgIndCh := GetPackageIndices(releasesCh)

	for pkgIndex := range pkgIndCh {
		pkgIndexSplits := strings.Split(string(pkgIndex.contents), "\n\n")
		fmt.Println("len = ", len(pkgIndexSplits))
		wg.Add(len(pkgIndexSplits))
		go func(c chan<- Package) {
			parsePackageIndex(pkgIndexSplits, "registry", "suite", pkgChan, &wg)
		}(pkgChan)
	}
	go func() {
		wg.Wait()
		close(pkgChan)
	}()
	return pkgChan
}

func GetPackagesForRelease(releases []ReleaseIndex) ([]Package, int) {
	var res []Package
	var pkgs chan Package = ApiGetPackages(releases)
	size := 0
	for pkg := range pkgs {
		size += 1
		res = append(res, pkg)
	}
	return res, size
}
