package gothub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/aktau/github-release/github"
	"github.com/voxelbrain/goptions"
)

type Options struct {
	Help      goptions.Help `goptions:"-h, --help, description='Show this help'"`
	Verbosity []bool        `goptions:"-v, --verbose, description='Be verbose'"`
	Quiet     bool          `goptions:"-q, --quiet, description='Do not print anything, even errors (except if --verbose is specified)'"`
	Version   bool          `goptions:"--version, description='Print version'"`

	goptions.Verbs
	Download struct {
		Token  string `goptions:"-s, --security-token, description='Github token ($GITHUB_TOKEN if set). required if repo is private.'"`
		User   string `goptions:"-u, --user, description='Github repo user or organisation (required if $GITHUB_USER not set)'"`
		Repo   string `goptions:"-r, --repo, description='Github repo (required if $GITHUB_REPO not set)'"`
		Latest bool   `goptions:"-l, --latest, description='Download latest release (required if tag is not specified)',mutexgroup='input'"`
		Tag    string `goptions:"-t, --tag, description='Git tag to download from (required if latest is not specified)', mutexgroup='input',obligatory"`
		Name   string `goptions:"-n, --name, description='Name of the file', obligatory"`
	} `goptions:"download"`
	Upload struct {
		Token   string   `goptions:"-s, --security-token, description='Github token (required if $GITHUB_TOKEN not set)'"`
		User    string   `goptions:"-u, --user, description='Github repo user or organisation (required if $GITHUB_USER not set)'"`
		Repo    string   `goptions:"-r, --repo, description='Github repo (required if $GITHUB_REPO not set)'"`
		Tag     string   `goptions:"-t, --tag, description='Git tag to upload to', obligatory"`
		Name    string   `goptions:"-n, --name, description='Name of the file', obligatory"`
		Label   string   `goptions:"-l, --label, description='Label (description) of the file'"`
		File    *os.File `goptions:"-f, --file, description='File to upload (use - for stdin)', rdonly, obligatory"`
		Replace bool     `goptions:"-R, --replace, description='Replace asset with same name if it already exists (WARNING: not atomic, failure to upload will remove the original asset too)'"`
	} `goptions:"upload"`
	Release struct {
		Token      string `goptions:"-s, --security-token, description='Github token (required if $GITHUB_TOKEN not set)'"`
		User       string `goptions:"-u, --user, description='Github repo user or organisation (required if $GITHUB_USER not set)'"`
		Repo       string `goptions:"-r, --repo, description='Github repo (required if $GITHUB_REPO not set)'"`
		Tag        string `goptions:"-t, --tag, obligatory, description='Git tag to create a release from'"`
		Name       string `goptions:"-n, --name, description='Name of the release (defaults to tag)'"`
		Desc       string `goptions:"-d, --description, description='Release description, use - for reading a description from stdin (defaults to tag)'"`
		Target     string `goptions:"-c, --target, description='Commit SHA or branch to create release of (defaults to the repository default branch)'"`
		Draft      bool   `goptions:"--draft, description='The release is a draft'"`
		Prerelease bool   `goptions:"-p, --pre-release, description='The release is a pre-release'"`
	} `goptions:"release"`
	Edit struct {
		Token      string `goptions:"-s, --security-token, description='Github token (required if $GITHUB_TOKEN not set)'"`
		User       string `goptions:"-u, --user, description='Github repo user or organisation (required if $GITHUB_USER not set)'"`
		Repo       string `goptions:"-r, --repo, description='Github repo (required if $GITHUB_REPO not set)'"`
		Tag        string `goptions:"-t, --tag, obligatory, description='Git tag to edit the release of'"`
		Name       string `goptions:"-n, --name, description='New name of the release (defaults to tag)'"`
		Desc       string `goptions:"-d, --description, description='New release description, use - for reading a description from stdin (defaults to tag)'"`
		Draft      bool   `goptions:"--draft, description='The release is a draft'"`
		Prerelease bool   `goptions:"-p, --pre-release, description='The release is a pre-release'"`
	} `goptions:"edit"`
	Delete struct {
		Token string `goptions:"-s, --security-token, description='Github token (required if $GITHUB_TOKEN not set)'"`
		User  string `goptions:"-u, --user, description='Github repo user or organisation (required if $GITHUB_USER not set)'"`
		Repo  string `goptions:"-r, --repo, description='Github repo (required if $GITHUB_REPO not set)'"`
		Tag   string `goptions:"-t, --tag, obligatory, description='Git tag of release to delete'"`
	} `goptions:"delete"`
	Info struct {
		Token string `goptions:"-s, --security-token, description='Github token ($GITHUB_TOKEN if set). required if repo is private.'"`
		User  string `goptions:"-u, --user, description='Github repo user or organisation (required if $GITHUB_USER not set)'"`
		Repo  string `goptions:"-r, --repo, description='Github repo (required if $GITHUB_REPO not set)'"`
		Tag   string `goptions:"-t, --tag, description='Git tag to query (optional)'"`
		JSON  bool   `goptions:"-j, --json, description='Emit info as JSON instead of text'"`
	} `goptions:"info"`
}

var (
	EnvToken       string
	EnvUser        string
	EnvRepo        string
	EnvApiEndpoint string
)

func Init() {
	EnvToken = os.Getenv("GITHUB_TOKEN")
	EnvUser = os.Getenv("GITHUB_USER")
	EnvRepo = os.Getenv("GITHUB_REPO")
	EnvApiEndpoint = os.Getenv("GITHUB_API")
}

var (
	VERBOSITY = 0
)

type Command func(Options) error

var Commands = map[goptions.Verbs]Command{
	"download": Downloadcmd,
	"upload":   Uploadcmd,
	"release":  Releasecmd,
	"edit":     Editcmd,
	"delete":   Deletecmd,
	"info":     Infocmd,
}

func Infocmd(opt Options) error {
	user := nvls(opt.Info.User, EnvUser)
	repo := nvls(opt.Info.Repo, EnvRepo)
	token := nvls(opt.Info.Token, EnvToken)
	tag := opt.Info.Tag

	if user == "" || repo == "" {
		return fmt.Errorf("user and repo need to be passed as arguments")
	}

	// Find regular git tags.
	foundTags, err := Tags(user, repo, token)
	if err != nil {
		return fmt.Errorf("could not fetch tags, %v", err)
	}
	if len(foundTags) == 0 {
		return fmt.Errorf("no tags available for %v/%v", user, repo)
	}

	tags := foundTags[:0]
	for _, t := range foundTags {
		// If the user only requested one tag, filter out the rest.
		if tag == "" || t.Name == tag {
			tags = append(tags, t)
		}
	}

	renderer := renderInfoText

	if opt.Info.JSON {
		renderer = renderInfoJSON
	}

	// List releases + assets.
	var releases []Release
	if tag == "" {
		// Get all releases.
		vprintf("%v/%v: getting information for all releases\n", user, repo)
		releases, err = Releases(user, repo, token)
		if err != nil {
			return err
		}
	} else {
		// Get only one release.
		vprintf("%v/%v/%v: getting information for the release\n", user, repo, tag)
		release, err := ReleaseOfTag(user, repo, tag, token)
		if err != nil {
			return err
		}
		releases = []Release{*release}
	}

	return renderer(tags, releases)
}

func renderInfoText(tags []Tag, releases []Release) error {
	fmt.Println("tags:")
	for _, tag := range tags {
		fmt.Println("-", &tag)
	}

	fmt.Println("releases:")
	for _, release := range releases {
		fmt.Println("-", &release)
	}

	return nil
}

func renderInfoJSON(tags []Tag, releases []Release) error {
	out := struct {
		Tags     []Tag
		Releases []Release
	}{
		Tags:     tags,
		Releases: releases,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(&out)
}

func Uploadcmd(opt Options) error {
	user := nvls(opt.Upload.User, EnvUser)
	repo := nvls(opt.Upload.Repo, EnvRepo)
	token := nvls(opt.Upload.Token, EnvToken)
	tag := opt.Upload.Tag
	name := opt.Upload.Name
	label := opt.Upload.Label
	file := opt.Upload.File

	vprintln("uploading...")

	if file == nil {
		return fmt.Errorf("provided file was not valid")
	}
	defer file.Close()

	if err := ValidateCredentials(user, repo, token, tag); err != nil {
		return err
	}

	// Find the release corresponding to the entered tag, if any.
	rel, err := ReleaseOfTag(user, repo, tag, token)
	if err != nil {
		return err
	}

	// If the user has attempted to upload this asset before, someone could
	// expect it to be present in the release struct (rel.Assets). However,
	// we have to separately ask for the specific assets of this release.
	// Reason: the assets in the Release struct do not contain incomplete
	// uploads (which regrettably happen often using the Github API). See
	// issue #26.
	var assets []Asset
	err = github.Client{Token: token, BaseURL: EnvApiEndpoint}.Get(fmt.Sprintf(ASSET_RELEASE_LIST_URI, user, repo, rel.Id), &assets)
	if err != nil {
		return err
	}

	// Incomplete (failed) uploads will have their state set to new. These
	// assets are (AFAIK) useless in all cases. The only thing they will do
	// is prevent the upload of another asset of the same name. To work
	// around this GH API weirdness, let's just delete assets if:
	//
	// 1. Their state is new.
	// 2. The user explicitly asked to delete/replace the asset with -R.
	if asset := findAsset(assets, name); asset != nil &&
		(asset.State == "new" || opt.Upload.Replace) {
		vprintf("asset (id: %d) already existed in state %s: removing...\n", asset.Id, asset.Name)
		if err := asset.Delete(user, repo, token); err != nil {
			return fmt.Errorf("could not replace asset: %v", err)
		}
	}

	v := url.Values{}
	v.Set("name", name)
	if label != "" {
		v.Set("label", label)
	}

	url := rel.CleanUploadUrl() + "?" + v.Encode()

	resp, err := github.DoAuthRequest("POST", url, "application/octet-stream",
		token, nil, file)
	if err != nil {
		return fmt.Errorf("can't create upload request to %v, %v", url, err)
	}
	defer resp.Body.Close()
	vprintln("RESPONSE:", resp)

	var r io.Reader = resp.Body
	if VERBOSITY != 0 {
		r = io.TeeReader(r, os.Stderr)
	}
	var asset *Asset
	// For HTTP status 201 and 502, Github will return a JSON encoding of
	// the (partially) created asset.
	if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusCreated {
		vprintf("ASSET: ")
		asset = new(Asset)
		if err := json.NewDecoder(r).Decode(&asset); err != nil {
			return fmt.Errorf("upload failed (%s), could not unmarshal asset (err: %v)", resp.Status, err)
		}
	} else {
		vprintf("BODY: ")
		if msg, err := ToMessage(r); err == nil {
			return fmt.Errorf("could not upload, status code (%s), %v",
				resp.Status, msg)
		}
		return fmt.Errorf("could not upload, status code (%s)", resp.Status)
	}

	if resp.StatusCode == http.StatusBadGateway {
		// 502 means the upload failed, but GitHub still retains metadata
		// (an asset in state "new"). Attempt to delete that now since it
		// would clutter the list of release assets.
		vprintf("asset (id: %d) failed to upload, it's now in state %s: removing...\n", asset.Id, asset.Name)
		if err := asset.Delete(user, repo, token); err != nil {
			return fmt.Errorf("upload failed (%s), could not delete partially uploaded asset (ID: %d, err: %v) in order to cleanly reset GH API state, please try again", resp.Status, asset.Id, err)
		}
		return fmt.Errorf("could not upload, status code (%s)", resp.Status)
	}

	return nil
}

func Downloadcmd(opt Options) error {
	user := nvls(opt.Download.User, EnvUser)
	repo := nvls(opt.Download.Repo, EnvRepo)
	token := nvls(opt.Download.Token, EnvToken)
	tag := opt.Download.Tag
	name := opt.Download.Name
	latest := opt.Download.Latest

	vprintln("downloading...")

	if err := ValidateTarget(user, repo, tag, latest); err != nil {
		return err
	}

	// Find the release corresponding to the entered tag, if any.
	var rel *Release
	var err error
	if latest {
		rel, err = LatestRelease(user, repo, token)
	} else {
		rel, err = ReleaseOfTag(user, repo, tag, token)
	}
	if err != nil {
		return err
	}

	asset := findAsset(rel.Assets, name)
	if asset == nil {
		return fmt.Errorf("coud not find asset named %s", name)
	}

	var resp *http.Response
	if token == "" {
		// Use the regular github.com site if we don't have a token.
		resp, err = http.Get("https://github.com" + fmt.Sprintf("/%s/%s/releases/download/%s/%s", user, repo, tag, name))
	} else {
		url := nvls(EnvApiEndpoint, github.DefaultBaseURL) + fmt.Sprintf(ASSET_URI, user, repo, asset.Id)
		resp, err = github.DoAuthRequest("GET", url, "", token, map[string]string{
			"Accept": "application/octet-stream",
		}, nil)
	}
	if err != nil {
		return fmt.Errorf("could not fetch releases, %v", err)
	}
	defer resp.Body.Close()

	vprintln("GET", resp.Request.URL, "->", resp)

	contentLength, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github did not respond with 200 OK but with %v", resp.Status)
	}

	out := os.Stdout // Pipe the asset to stdout by default.
	if isCharDevice(out) {
		// If stdout is a char device, assume it's a TTY (terminal). In this
		// case, don't pipe th easset to stdout, but create it as a file in
		// the current working folder.
		if out, err = os.Create(name); err != nil {
			return fmt.Errorf("could not create file %s", name)
		}
		defer out.Close()
	}

	return mustCopyN(out, resp.Body, contentLength)
}

// mustCopyN attempts to copy exactly N bytes, if this fails, an error is
// returned.
func mustCopyN(w io.Writer, r io.Reader, n int64) error {
	an, err := io.Copy(w, r)
	if an != n {
		return fmt.Errorf("data did not match content length %d != %d", an, n)
	}
	return err
}

func ValidateTarget(user, repo, tag string, latest bool) error {
	if user == "" {
		return fmt.Errorf("empty user")
	}
	if repo == "" {
		return fmt.Errorf("empty repo")
	}
	if tag == "" && !latest {
		return fmt.Errorf("empty tag")
	}
	return nil
}

func ValidateCredentials(user, repo, token, tag string) error {
	if err := ValidateTarget(user, repo, tag, false); err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("empty token")
	}
	return nil
}

func Releasecmd(opt Options) error {
	cmdopt := opt.Release
	user := nvls(cmdopt.User, EnvUser)
	repo := nvls(cmdopt.Repo, EnvRepo)
	token := nvls(cmdopt.Token, EnvToken)
	tag := cmdopt.Tag
	name := nvls(cmdopt.Name, tag)
	desc := nvls(cmdopt.Desc, tag)
	target := nvls(cmdopt.Target)
	draft := cmdopt.Draft
	prerelease := cmdopt.Prerelease

	vprintln("releasing...")

	if err := ValidateCredentials(user, repo, token, tag); err != nil {
		return err
	}

	// Check if we need to read the description from stdin.
	if desc == "-" {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("could not read description from stdin: %v", err)
		}
		desc = string(b)
	}

	params := ReleaseCreate{
		TagName:         tag,
		TargetCommitish: target,
		Name:            name,
		Body:            desc,
		Draft:           draft,
		Prerelease:      prerelease,
	}

	/* encode params as json */
	payload, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("can't encode release creation params, %v", err)
	}
	reader := bytes.NewReader(payload)

	URL := nvls(EnvApiEndpoint, github.DefaultBaseURL) + fmt.Sprintf("/repos/%s/%s/releases", user, repo)
	resp, err := github.DoAuthRequest("POST", URL, "application/json", token, nil, reader)
	if err != nil {
		return fmt.Errorf("while submitting %v, %v", string(payload), err)
	}
	defer resp.Body.Close()

	vprintln("RESPONSE:", resp)
	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == 422 {
			return fmt.Errorf("github returned %v (this is probably because the release already exists)",
				resp.Status)
		}
		return fmt.Errorf("github returned %v", resp.Status)
	}

	if VERBOSITY != 0 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error while reading response, %v", err)
		}
		vprintln("BODY:", string(body))
	}

	return nil
}

func Editcmd(opt Options) error {
	cmdopt := opt.Edit
	user := nvls(cmdopt.User, EnvUser)
	repo := nvls(cmdopt.Repo, EnvRepo)
	token := nvls(cmdopt.Token, EnvToken)
	tag := cmdopt.Tag
	name := nvls(cmdopt.Name, tag)
	desc := nvls(cmdopt.Desc, tag)
	draft := cmdopt.Draft
	prerelease := cmdopt.Prerelease

	vprintln("editing...")

	if err := ValidateCredentials(user, repo, token, tag); err != nil {
		return err
	}

	id, err := IdOfTag(user, repo, tag, token)
	if err != nil {
		return err
	}

	vprintf("release %v has id %v\n", tag, id)

	// Check if we need to read the description from stdin.
	if desc == "-" {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("could not read description from stdin: %v", err)
		}
		desc = string(b)
	}

	/* the release create struct works for editing releases as well */
	params := ReleaseCreate{
		TagName:    tag,
		Name:       name,
		Body:       desc,
		Draft:      draft,
		Prerelease: prerelease,
	}

	/* encode the parameters as JSON, as required by the github API */
	payload, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("can't encode release creation params, %v", err)
	}

	URL := nvls(EnvApiEndpoint, github.DefaultBaseURL) + fmt.Sprintf("/repos/%s/%s/releases/%d", user, repo, id)
	resp, err := github.DoAuthRequest("PATCH", URL, "application/json", token, nil, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("while submitting %v, %v", string(payload), err)
	}
	defer resp.Body.Close()

	vprintln("RESPONSE:", resp)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 422 {
			return fmt.Errorf("github returned %v (this is probably because the release already exists)",
				resp.Status)
		}
		return fmt.Errorf("github returned unexpected status code %v", resp.Status)
	}

	if VERBOSITY != 0 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error while reading response, %v", err)
		}
		vprintln("BODY:", string(body))
	}

	return nil
}

func Deletecmd(opt Options) error {
	user, repo, token, tag := nvls(opt.Delete.User, EnvUser),
		nvls(opt.Delete.Repo, EnvRepo),
		nvls(opt.Delete.Token, EnvToken),
		opt.Delete.Tag
	vprintln("deleting...")

	id, err := IdOfTag(user, repo, tag, token)
	if err != nil {
		return err
	}

	vprintf("release %v has id %v\n", tag, id)

	baseURL := nvls(EnvApiEndpoint, github.DefaultBaseURL)
	resp, err := github.DoAuthRequest("DELETE", baseURL+fmt.Sprintf("/repos/%s/%s/releases/%d",
		user, repo, id), "application/json", token, nil, nil)
	if err != nil {
		return fmt.Errorf("release deletion failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("could not delete the release corresponding to tag %s on repo %s/%s",
			tag, user, repo)
	}

	return nil
}
