package main

import (
	"embed"
	"go-templ/api"
	"net/http"
	"os"
	"runtime/pprof"
	"sort"
	"strconv"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

var releasePaths = []string{
	"http://archive.ubuntu.com/ubuntu/dists/bionic/Release",
	"http://archive.ubuntu.com/ubuntu/dists/jammy/Release",
	"http://archive.ubuntu.com/ubuntu/dists/focal/Release",
	"http://archive.ubuntu.com/ubuntu/dists/devel/Release",
	"http://archive.ubuntu.com/ubuntu/dists/lunar/Release",
	"http://archive.ubuntu.com/ubuntu/dists/mantic/Release",
}

var releaseList = []api.ReleaseIndex{
	// {
	// 	ReleaseUrl: "http://archive.ubuntu.com/ubuntu/dists/bionic/Release",
	// 	Registry:   "http://archive.ubuntu.com/ubuntu/",
	// 	Suite:      "bionic",
	// },
	// {
	// 	ReleaseUrl: "http://archive.ubuntu.com/ubuntu/dists/jammy/Release",
	// 	Registry:   "http://archive.ubuntu.com/ubuntu/",
	// 	Suite:      "jammy",
	// },
	// {
	// 	ReleaseUrl: "http://archive.ubuntu.com/ubuntu/dists/focal/Release",
	// 	Registry:   "http://archive.ubuntu.com/ubuntu/",
	// 	Suite:      "focal",
	// },
	// {
	// 	ReleaseUrl: "http://archive.ubuntu.com/ubuntu/dists/devel/Release",
	// 	Registry:   "http://archive.ubuntu.com/ubuntu/",
	// 	Suite:      "devel",
	// },
	// {
	// 	ReleaseUrl: "http://archive.ubuntu.com/ubuntu/dists/lunar/Release",
	// 	Registry:   "http://archive.ubuntu.com/ubuntu/",
	// 	Suite:      "lunar",
	// },
	{
		ReleaseUrl: "http://archive.ubuntu.com/ubuntu/dists/mantic/Release",
		Registry:   "http://archive.ubuntu.com/ubuntu/",
		Suite:      "mantic",
	},
}

const (
	DefaultPrefix = "/debug/pprof"
)

type PkgContainer struct {
	pkgs []api.Package
}

var container PkgContainer

func getPrefix(prefixOptions ...string) string {
	if len(prefixOptions) > 0 {
		return prefixOptions[0]
	}
	return DefaultPrefix
}

func handler(h http.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		h.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	}
}

func index(c echo.Context) error {
	dat, err := os.ReadFile("./index.html")
	if err != nil {
		panic(err)
	}
	return c.HTMLBlob(200, dat)
}

func postClick(c echo.Context) error {
	return Render(c, 200, hello("Templar assasin"))
}

func getPkgs(c echo.Context) error {
	ps := container.pkgs[:100]
	return Render(c, 200, pkgs(ps))
}

func releases(c echo.Context) error {
	return Render(c, 200, pkgs(container.pkgs[10:20]))
}

func apiReleases(c echo.Context) error {
	c.Response().Writer.Write(api.ApiReleases(releasePaths))
	return nil
}

func apiRelease(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		panic(err)
	}
	c.Response().Writer.Write(api.CollectRelease(releasePaths[id]))
	return nil
}

func apiAvailableReleases(c echo.Context) error {
	c.Response().Writer.Write(api.ApiAvailableReleases(releasePaths))
	return nil
}

func apiPackages(c echo.Context) error {
	_, size := api.GetPackagesForRelease(releaseList)
	// c.Response().Writer.Write(response)
	c.Response().Writer.Write([]byte("\n" + "size:" + strconv.Itoa(size) + "\n"))
	return nil
}

func release(c echo.Context) error {
	ind := api.CollectRelease(releasePaths[0])
	return Render(c, 200, hello(string(ind)))
}

func Render(ctx echo.Context, statusCode int, t templ.Component) error {
	ctx.Response().Writer.WriteHeader(statusCode)
	ctx.Response().Header().Add(echo.HeaderContentType, echo.MIMETextHTML)
	return t.Render(ctx.Request().Context(), ctx.Response().Writer)
}

//go:embed static
var static embed.FS

//go:generate npm run build
func main() {
	e := echo.New()
	assetHandler := http.FileServer(http.FS(static))
	// e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", assetHandler)))
	e.GET("/static/*", echo.WrapHandler(assetHandler))

	e.GET("/", index)
	e.POST("/clicked", postClick)
	e.GET("/pkgs", getPkgs)
	e.GET("/releases", releases)
	e.GET("/api/releases", apiReleases)
	e.GET("/api/releases/available", apiAvailableReleases)
	e.GET("/api/releases/:id", apiReleases)
	e.GET("/api/packages", apiPackages)
	e.GET("/release", release)

	var pkgs, _ = api.GetPackagesForRelease(releaseList)

	var pkgsByname PackagesByName = pkgs
	sort.Sort(pkgsByname)

	container = PkgContainer{
		pkgs: pkgsByname,
	}

	f, _ := os.Create("myprogram.prof")
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	e.Logger.Fatal(e.Start("localhost:1323"))
}

type PackagesByName []api.Package

func (a PackagesByName) Len() int           { return len(a) }
func (a PackagesByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PackagesByName) Less(i, j int) bool { return a[i].Name < a[j].Name }
