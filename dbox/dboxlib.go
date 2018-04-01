// dboxlib implements a client for the dropbox v2 api
package dboxlib

import(
	"os"
	"os/exec"
	"time"
	"net/http"
	"net/url"
	"io"
	"io/ioutil"
	"fmt"
	"log"
	"sort"
	"strings"
	"bytes"
	"strconv"
	"menteslibres.net/gosexy/yaml"
	"menteslibres.net/gosexy/to"
	"encoding/json"
	"github.com/cheggaaa/pb"
	"runtime"
	pth "path"
	ospath "path/filepath"
	"sync"
	"github.com/xiconet/utils"
	fd "github.com/xiconet/godownload"
)

const(
	api_url = "https://api.dropbox.com/2"
	content_url = "https://content.dropboxapi.com/2"
	cfg_file = "C:/Users/ARC/.config/jagoodrop.yml"
)

var(
	users = map[string]string{"xiconet": "0", "jcoxi": "1", "jcoxinet": "2", "coxinet": "3"}
	audioTypes = []string{".mp3", ".flac", ".ape", ".wav", ".wv", ".mpc", ".ogg", ".m4a"}
	unhandled = []string{".ape", ".wv", ".wav"}
	Chunksize = int64(8*1024*1024)
)

func Userlist() (u []string) {
	for user, _ := range users {
		u = append(u, user)
	}
	return
}

func Uids() (u []string) {
	for _, uid := range users {
		u = append(u, uid)
	}
	return
}

func UidToUser(u string) (string, error) {
	for user, uid := range users {
		if u == uid {
			return user, nil
		}
	}
	return "", fmt.Errorf("unregistered user")
}

type Info struct {
		AccountId string `json:"account_id"`
		Name struct {
				GivenName 		string `json:"given_name"`
				Surname 		string `json:"surname"`
				FamiliarName 	string `json:"familiar_name"`
				DisplayName 	string `json:"display_name"`
				AbbreviatedName string `json:"abbreviated_name"`
		} `json:"name"` 
		Email 			string 	`json:"email"`
		EmailVerified 	bool   	`json:"email_verified"`
		Disabled 		bool 	`json:"disabled"`
		Country       	string 	`json:"country"`
		Locale 			string 	`json:"locale"`
		ReferralLink 	string 	`json:"referral_link"`
		IsPaired 		bool 	`json:"is_paired"`
		AccountType struct {
				Tag string `json:".tag"`
		} `json:"account_type"`
		RootInfo struct  {
				Tag 			string `json:".tag"`
				RootNamespaceId string `json:"root_namespace_id"`
				HomeNamespaceId string `json:"home_namespace_id"`
		} `json:"root_info"`
}

type SpaceUsage struct {
		Used int64 `json:"used"`
		Allocation struct {
				Tag string `json:".tag"`
				Individual string `json:"individual"`
				Allocated int64 `json:"allocated"`
		} `json:"allocation"`
}

type Entry struct {
		Tag string `json:".tag"`
		Name string `json:"name"`
		PathLower string `json:"path_lower"`
		PathDisplay string `json:"path_display"`
		Id string `json:"id"`
		Size int64 `json:size"`
		User string // to be set later on
}

type Entries []Entry 
type ByName []Entry

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }


func (s Entries) SetUser(user string, i int) {
	s[i].User = user
}

type DboxFolder struct {
		Entries Entries `json:"entries"`
		Cursor  string  `json:"cursor"`
		HasMore bool    `json:"has_more"`
}

type Meta struct {
		ClientModified	string	`json:"client_modified"`
		ContentHash	string 		`json:"content_hash"`
		Id	string				`json:"id"` 
		Name string 			`json:"name"` 
		PathLower string 		`json:"path_lower"`
		PathDisplay string		`json:"path_display"`
		Rev string 				`json:"rev"`		
		ServerModified string	`json:"server_modified"`		
		Size int64				`json:"size"`
		Tag string 				`json:".tag"`
		ErrorSummary string     `json:"error_summary,omitempty"`
		Error DbxError          `json:"error,omitempty"`		
		User string				// to be set later on 
}

type DbxError struct  {
		Tag string `json:".tag,omitempty"`
				Path struct {
						Tag string `json:".tag,omitempty"`
						Conflict struct  {
								Tag string `json:".tag,omitempty"`
						} `json:"conflict,omitempty"`
		} `json:"path,omitempty"` 
}

//create folder response
type FolderMeta struct {
		Metadata struct {
				Id          string  `json:"id"`
				Name        string  `json:"name"`
				PathDisplay string  `json:"path_display"`
				PathLower   string  `json:"path_lower"`
		} `json:"metadata"`
		ErrorSummary string `json:"error_summary,omitempty"`
		Error DbxError `json:"error,omitempty"`
}


type Metaset []Meta
func (s Metaset) SetUser(user string, i int) {
	s[i].User = user
}

type Item struct {
		Bytes       int64  `json:"bytes"`
		Icon        string `json:"icon"`
		IsDeleted   bool   `json:"is_deleted"`
		IsDir       bool   `json:"is_dir"`
		Modified    string `json:"modified"`
		Path        string `json:"path"`
		ReadOnly    bool   `json:"read_only"`
		Rev         string `json:"rev"`
		Revision    int    `json:"revision"`
		Root        string `json:"root"`
		Size        string `json:"size"`
		ThumbExists bool   `json:"thumb_exists"`
		User        string
}


type Dbox struct {
        Bytes    	int64 	`json:"bytes"`
        Contents    []Item 	`json:"contents"`
        Hash        string  `json:"hash"`
        Icon        string  `json:"icon"`
        IsDir       bool    `json:"is_dir"`
        Modified    string  `json:"modified"`
        Path        string  `json:"path"`
        ReadOnly    bool    `json:"read_only"`
        Rev         string  `json:"rev"`
        Revision    int     `json:"revision"`
        Root        string  `json:"root"`
        Size        string  `json:"size"`
        ThumbExists bool    `json:"thumb_exists"`
}

type ByPath []Item

func (a ByPath) Len() int           { return len(a) }
func (a ByPath) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPath) Less(i, j int) bool { return a[i].Path < a[j].Path }

type Itemset []Item
func (s Itemset) SetUser(user string, i int) {
	s[i].User = user
}

type Cursor struct {
		SessionId string `json:"session_id"`
		Offset     int64 `json:"offset"`
}

type Client struct {
		BaseUrl string
		CfgFile string
		User string
		Auth Auth
		Endpoints map[string]string		
}

type Auth struct {
		TokenType string
		Token     string
}

func NewClient(baseUrl, cfgFile, user string, auth Auth, endpoints map[string]string) (c *Client){
	return &Client{baseUrl, cfgFile, user, Auth{}, map[string]string{}}
}

func (c *Client) SetToken(user string) string {
	config, err := yaml.Open(cfg_file)
    if err != nil {
        panic(err)
    }
	if user == "current_user" {
		user = to.String(config.Get("users", "current_user"))
	}
	token := to.String(config.Get(user, "access_token"))
	c.User = user
	c.Auth.TokenType = "Bearer"
	c.Auth.Token = token	
	return token	
}

//get user account information
func (c *Client) Info(){
	status, body := c.apiRequest("POST", "/users/get_current_account", nil, nil, false)
	if status != "200 OK" {
		fmt.Println("error: bad server status:", status)
		fmt.Println(string(body))
		os.Exit(1)
	} 
    info := Info{}
	err := json.Unmarshal([]byte(body), &info)
	if err != nil {fmt.Println(err); os.Exit(1)}
	
	status, body = c.apiRequest("POST", "/users/get_space_usage", nil, nil, false)
	if status != "200 OK" {
		fmt.Println("error: bad server status:", status)
		fmt.Println(string(body))
		os.Exit(1)
	}
	usage := SpaceUsage{}
	err = json.Unmarshal([]byte(body), &usage)
	if err != nil {fmt.Println(err); os.Exit(1)}
	
	left, _  := utils.NiceBytes(usage.Allocation.Allocated - usage.Used)
	quota, _ := utils.NiceBytes(usage.Allocation.Allocated)
	used, _  := utils.NiceBytes(usage.Used)
	fmt.Printf("Email: %s\nDisplay name: %s\nAccount Id: %s\n", info.Email, info.Name.DisplayName, info.AccountId)
	fmt.Printf("Quota: %d [%s]\n", usage.Allocation.Allocated, quota)
	fmt.Printf("Used: %d [%s]\n", usage.Used, used)
	fmt.Printf("Left space: %d byes [%s]\n", usage.Allocation.Allocated - usage.Used, left)
}

// generic api request
func (c *Client) apiRequest(method, endpoint string, params interface{}, data interface{}, isJson bool) (string, []byte) {
	uri, err := url.Parse(c.BaseUrl)
	if err != nil {fmt.Println(err); os.Exit(1)}
	uri.Path += endpoint
	if params != nil {
		p := params.(map[string]string)
		q := uri.Query()
		for k, v := range p {
			q.Set(k, v)
		}
		uri.RawQuery = q.Encode()
	}
	var req *http.Request
	if data != nil {
		if isJson {
			form := data.(map[string]string)
			form_js, _ := json.Marshal(form)
			req, err = http.NewRequest(method, uri.String(), strings.NewReader(string(form_js)))
		} else {	
			form := data.(url.Values)
			req, err = http.NewRequest(method, uri.String(), strings.NewReader(form.Encode()))
		}
	} else {
		req, err = http.NewRequest(method, uri.String(), nil)
	}
	if err != nil {fmt.Println(err); os.Exit(1)}
	req.Header.Set("Authorization", "Bearer "+c.Auth.Token)
	if method == "POST" || method == "PUT" {
		if isJson {
			req.Header.Add("Content-Type", "application/json")
		} else if data != nil {	
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(1)		
	}
	defer resp.Body.Close()
	switch {
	  case method == "POST":
		if resp.StatusCode != 200 {
			if resp.StatusCode == 403 {
				fmt.Println("server status for %q request: %s", method, resp.Status)
			} else {
				fmt.Printf("error: bad server status for %q request: %s\n", method, resp.Status)
			}
		}
	  default: 
		if resp.StatusCode/10 != 20 {						
			fmt.Printf("error: bad server status for %q request: %s\n", method, resp.Status)
			fmt.Printf("request: %+v\n", req)
		}
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {fmt.Println(err); os.Exit(1)}	
	return resp.Status, body
}

func (c *Client) GetMetadata(path string) (meta Meta, err error){
	ep := "/files/alpha/get_metadata"
	params := map[string]string{"path":path} 
	isJson := true
	status, body := c.apiRequest("POST", ep, nil, params, isJson)
	if status != "200 OK" {
		if strings.Contains(status, "path/not_file/"){
			meta.Tag = "folder"
		} else {
			err = fmt.Errorf("error: bad server status:", status+"\n"+string(body))
		} 
		return
	} else {
		err = json.Unmarshal([]byte(body), &meta)
		return
	}
}

func fromJson(body []byte) (data DboxFolder) {
	err := json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("error while decoding response body:", err)
		fmt.Println("body", string(body))
	}
	return
}

func printRes(contents Entries) {
	sort.Sort(ByName(contents))
	for _, v := range contents {
		if v.Size == 0 {
			fmt.Println(v.Name)
		} else {
			filesize, _ := utils.NiceBytes(v.Size)
			fmt.Printf("%-68s %8s\n", v.Name, filesize)
		}
	}
}

func Legend(users map[string]string) string {
	legend := make([]string, len(users))
	i := 0
	for k, v := range users {
		legend[i] = fmt.Sprintf("[%s]=%s" , v, k)
		i += 1
	}
	return strings.Join(legend, ", ")
}

func printCompiled(contents []Entry) {
	for _, v := range contents {
		name := v.Name
		userId := users[v.User]
		if v.Size == 0 {
			fmt.Printf("[%s] %s\n", userId, name)
		} else {
			size, _ := utils.NiceBytes(v.Size)
			fmt.Printf("[%s] %-65s %s\n", userId, name, size)
		}
	}	
	fmt.Printf("\n[%s]\n", Legend(users))
}

func (c *Client) getResource(path string) (data DboxFolder) {
	ep := "/files/list_folder"
	params := map[string]string{"path":path}
	isJson := true
	status, body := c.apiRequest("POST", ep, nil, params, isJson)
	if status != "200 OK" {
		fmt.Println("error: bad server status:", status+"\n"+string(body))
	} else {
		//fmt.Println(string(body))
		data = fromJson(body)	
	}	
	return
}

func (c *Client) ListFolder(path string){	
	fmt.Printf("User: %s\n", c.User)
	res := c.getResource(path)
	printRes(res.Entries)
}


func (c *Client) GetTree(path string, depth, d int) {
	mx := 69
	i := strings.Repeat("  ", d)
	entries := c.getResource(path).Entries
	sort.Sort(ByName(entries))
	for _, e := range entries {
		name := e.Name
		if e.Size != 0 {
			if len(name) + len(i) > mx {
				name = utils.Shorten(name, mx - len(i))
			}
			if len(name) + len(i) < mx {				
				name = utils.RightPad(name, " ", mx - (len(name) + len(i)))
			}
			fmt.Printf("%s%s %d\n", i, name, e.Size)
		} else {
			fmt.Printf("%s%s\n", i, name)
			if depth == 0 || d+1 < depth {
				c.GetTree(e.PathLower, depth, d+1)
			}
		}
	}
}
	
func (c *Client) ListAll(path string) {
	var compiled []Entry
	for user, _ := range users {  		
		c.SetToken(user)
		res := c.getResource(path)
		var data Entries
		data = res.Entries
		for k, _ := range data { data.SetUser(user, k) }
		compiled = append(data, compiled...) 		
	}
	sort.Sort(ByName(compiled))
	printCompiled(compiled)
}
	
//type Link struct {
//		Expires string `json:"expires"`
//		Url     string `json:"url"`
//}

type Link struct {
    Metadata Meta `json "metadata"`
    Link string `json:"link"`
}
	
// get a streamable link to a file
func (c *Client) getLink(path string) (link Link, err error) {
    ep := "/files/get_temporary_link"
	data := map[string]string{"path":path}
	status, body := c.apiRequest("POST", ep, nil, data, true)
    if status != "200 OK" {
	   err = fmt.Errorf("error: bad server status: "+status+"\n"+string(body))
	   return 
	}
    err = json.Unmarshal(body, &link)
	return	 
}

func (c *Client) GetLinks(path string, stream bool) [][]string {
    links := [][]string{}
	meta, err := c.GetMetadata(path)
	if err != nil {fmt.Println(err); os.Exit(2)}	
	if meta.Tag == "folder" {	
		folderName := pth.Base(path)
		res := c.getResource(path)
		items := res.Entries
		sort.Sort(ByName(items))
		for _, i := range items{
			if i.Tag != "folder" {
				if stream && !isAudioExt(pth.Ext(i.Name)) {
					continue 
				}
				filePath := pth.Join(folderName, i.Name)
				if link, err := c.getLink(i.PathDisplay); err == nil {
					links = append(links, []string{filePath, link.Link})
				}
			}
		}
	} else {
		fileName := pth.Base(path)
		if link, err := c.getLink(path); err == nil {
			links = append(links, []string{fileName, link.Link})
		}
	}	
	if !stream {
		for _, k := range links {
			fmt.Println(k)
		}
	}
	return links
}

func isAudioExt(p string) bool {
	return utils.StringInSlice(pth.Ext(p), audioTypes)	
}

func (c *Client) StreamLinks(path string) {
    vlc := "C:/Program Files (x86)/VideoLAN/VLC/vlc.exe"
	fb2k := "C:/Program Files (x86)/foobar2000/foobar2000.exe"
	playerName := map[string]string{vlc: "VLC", fb2k: "foobar2000"}
	
	links := c.GetLinks(path, true)

	if len(links) == 0 {
		fmt.Printf("didn't find any registered audio type file in %s", path)
		os.Exit(0)
	}
	
    player := fb2k
	for _, k := range links {
		if utils.StringInSlice(pth.Ext(k[1]), unhandled) {
			player = vlc 
			break
		}
	}
	fmt.Printf("got %d files\n", len(links))
	var args []string	
	if player == fb2k { args = append(args, "/add")}
    for _, f := range links {
		fmt.Println(f[0])
		args = append(args, f[1])
	}
	if player == vlc { args = append(args, "--qt-start-minimized") }		
	fmt.Println("\nlaunching", playerName[player])	
    cmd := exec.Command(player, args...) 		 
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr		 
	err := cmd.Start()
	if err != nil {log.Fatal(err)}  
}

func (c *Client) downloadFile(itemPath, filepath string, aria, fast bool, conns int) int64 {
	uri := "https://content.dropboxapi.com/2/files/download"
	auth := fmt.Sprintf("%s %s", c.Auth.TokenType, c.Auth.Token)
	p := map[string]string{"path":itemPath}
	params, _ := json.Marshal(p)
	switch {
	case fast:
		uri += fmt.Sprintf("?access_token=%s", c.Auth.Token)
		c.fastFileDownload(uri, conns, filepath)
		return 0
	case aria :	
		cmd := exec.Command("aria2c" , "--file-allocation=falloc", "--max-connection-per-server=5" , "--min-split-size=1M", "--remote-time=true", "--header=Authorization:"+auth, "--header=Dropbox-API-Arg:"+string(params), uri, "-o "+filepath)		
		cmd.Stdout = os.Stdout		
		cmd.Stderr = os.Stderr				
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		stat, err := os.Stat(filepath)
		if err != nil {panic(err)}
		return stat.Size()
		
	default:
		req, err := http.NewRequest("GET", uri, nil)		
		req.Header.Add("Authorization", auth)
		req.Header.Add("Dropbox-API-Arg", string(params))	
		client := &http.Client{}
		resp, err := client.Do(req)	
		if err != nil { 
			panic(err) 
		}		
		if resp.StatusCode != 200 {
			fmt.Println("error: bad server status:", resp.Status)
			return 0 
		}		
		defer resp.Body.Close()					
		out, err := os.Create(filepath)
		if err != nil {panic(err)}
		defer out.Close()				
		i, _ := strconv.Atoi(resp.Header.Get("Content-Length"))	
		sourceSize := int64(i)
		source := resp.Body		
		bar := pb.New(int(sourceSize)).SetUnits(pb.U_BYTES).SetRefreshRate(time.Millisecond * 10)
		if sourceSize >= 1024 * 1024 {
			bar.ShowSpeed = true
		}
		bar.Start()
		writer := io.MultiWriter(out, bar)
		n, err := io.Copy(writer, source)	
		if err != nil {
			fmt.Println("Error while downloading", uri, "-", err)
			return 0
		}
		bar.Finish()
		return n
	}
}


func (c *Client) downsync(path, folderpath string, aria, fast bool, depth, r, conns int) {
	err := os.MkdirAll(folderpath, 0777)
	if err != nil {
		fmt.Println("error: could not create new folder", folderpath, ":", err)
		os.Exit(2)
	}	
	res := c.getResource(path)
	items := res.Entries	
	for _, e := range items {
		if e.Tag != "folder" {
			filepath := folderpath + "/" + e.Name
			fmt.Println("downloading", filepath)
			var dlfast bool 
			if fast && e.Size >= 1024*1024 {
				dlfast = true 
			}
			n := c.downloadFile(e.PathDisplay, filepath, aria, dlfast, conns)
			if !fast && (n != e.Size) {
				fmt.Printf("error: size mismatch, expected: %d bytes, actual: %d bytes\n", e.Size, n)
			}
		} 
		if (r < depth || depth == 0) && (e.Tag == "folder") {
			c.downsync(e.PathDisplay, folderpath+"/"+e.Name , aria, fast, depth, r+1, conns)
		}
	}
}


func (c *Client) Download(path, localPath string, aria, fast bool, depth, parallel, conns int) {

	meta, err := c.GetMetadata(path)
	if err != nil {fmt.Println(err); os.Exit(2)}	
	if meta.Tag != "folder" {
		c.downloadFile(path, localPath, aria, fast, conns)
	} else {
		data := c.getResource(path)
		if parallel > 0 {
			items := data.Entries
			c.parallelDownload(items, localPath, parallel)
				
		} else {
			c.downsync(path, localPath, aria, fast, depth, 1, conns)
		}
	}
}

func (c *Client) fastFileDownload(url string, conns int, outfile string) {
	d := fd.New()
	size, filename, err := d.Init(url, conns, outfile)
	var filesize string
	if f, err := utils.NiceBytes(int64(size)); err == nil {
		filesize = f 
	}
	fmt.Printf("File size: %s; filename: %s\n", filesize, filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	d.StartDownload()
	go d.Wait()
	DisplayProgress(&d)
}

func DisplayProgress(dl *fd.Downloader) {
	barWidth := float64(37)
	for {
		status, total, downloaded, elapsed := dl.GetProgress()
		frac := float64(downloaded)/float64(total)
		bps, _ := utils.NiceBytes(int64(float64(downloaded)/elapsed.Seconds()))
		tot, _ := utils.NiceBytes(int64(total))
		if frac == 0 { continue }
		fmt.Fprintf(os.Stdout, "[%-38s] %5.1f%% of %10s %10s/s %3.fs\r", 
					strings.Repeat("=", int(frac*barWidth))+">", frac*100, tot, bps, elapsed.Seconds())
		switch {
		case status == fd.Completed:
			fmt.Println("\nDownload successfully completed in", elapsed)
			return
		case status == fd.OnProgress: // needed?
		case status == fd.NotStarted: // needed?
		default:
			fmt.Printf("\nDownload failed: %s\n", status)
			os.Exit(1)
		}
		time.Sleep(time.Second)
	}
}

func (c *Client) parallelDownload(items []Entry, localFolder string, p int){
	var d, k int
	var dbytes int64
	err := os.Mkdir(localFolder, 0777)
	if err != nil {panic(err)}
	s := time.Now().Unix()
	for k < len(items) {
		var wg sync.WaitGroup
		for d = 0; d < p; d += 1 {
			if k+d < len(items) && items[k+d].Tag != "folder" {
				wg.Add(1)
				f := items[k+d]
				// anonymous func can be replaced by a named one 
				// by passing a pointer to wg.SyncGroup
				// see example in "parallel_downloads.go"
				go func(f Entry) {
					defer wg.Done()
					uri := content_url + "/files/download"
					req, _ := http.NewRequest("GET", uri, nil)
					auth := fmt.Sprintf("%s %s", c.Auth.TokenType, c.Auth.Token)					
					req.Header.Set("Authorization", auth)
					p := map[string]string{"path":f.PathDisplay}
					params, _ := json.Marshal(p)
					req.Header.Set("Dropbox-API-Arg", string(params))
					fmt.Println("downloading:", f.PathDisplay)
					resp, _ := http.DefaultClient.Do(req)
					defer resp.Body.Close()
					fmt.Println(resp.Status)
					fp := ospath.Join(localFolder, f.Name)
					out, err := os.Create(fp)
					if err != nil {panic(err)}
					defer out.Close()
					n, _ := io.Copy(out, resp.Body)	
					dbytes += n
				}(f)
			}
		}
		wg.Wait()
		k += d
	}
	dtime := time.Now().Unix() - s
	fmt.Printf("downloaded %d bytes in %d seconds\n", dbytes, dtime)
}

func (c *Client) getFile(f Entry, wg *sync.WaitGroup, localFolder string) { 
	p := map[string]string{"path": f.PathDisplay}
	uri := content_url + "/files/download"
	req, _ := http.NewRequest("GET", uri, nil)
	auth := fmt.Sprintf("%s %s", c.Auth.TokenType, c.Auth.Token)					
	req.Header.Set("Authorization", auth)
	params, _ := json.Marshal(p)
	req.Header.Set("Dropbox-API-Arg", string(params))
	fmt.Println("downloading:", f.PathDisplay)
	resp, _ := http.DefaultClient.Do(req)
    defer resp.Body.Close()

	fmt.Println(resp.Status)
    filename := ospath.Base(f.PathDisplay)
    out, err := os.Create(ospath.Join(localFolder, filename))
    if err != nil {
        panic(err)
    }
    defer out.Close()
    io.Copy(out, resp.Body)
    wg.Done()
}


func (c *Client) pipedUpload(localPath, parent string) {
	filename := ospath.Base(localPath)
	remotePath := pth.Join(parent, filename)
	if remotePath[:1] != "/" {
		remotePath = "/" + remotePath
	}
    url := "https://content.dropboxapi.com/2/files/upload"
	p := map[string]string{"path":remotePath}
	params, _ := json.Marshal(p)
	input, err := os.Open(localPath)
	check(err)
	defer input.Close()
	stat, err := input.Stat()
	check(err)
	pipeOut, pipeIn := io.Pipe()
	fsize := stat.Size()
	bar := pb.New(int(fsize)).SetUnits(pb.U_BYTES)
	if fsize >= 1024 {
		bar.ShowSpeed = true
	}
	writer := io.Writer(pipeIn)
	// do the request concurrently
	var resp *http.Response
	done := make(chan error)
	
	go func() {
		req, err := http.NewRequest("POST", url, pipeOut)
		if err != nil {
			done <- err
			return
		}
		req.ContentLength = fsize 
		req.Header.Set("Authorization", "Bearer "+c.Auth.Token)
		req.Header.Set("Dropbox-API-Arg", string(params))
		req.Header.Set("Content-Type", "application/octet-stream")
		log.Println("Created Request")
		bar.Start()
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			done <- err
			return
		}
		done <- nil
	}()	
	out := io.MultiWriter(writer, bar)
	_, err = io.Copy(out, input)
	check(err)
	check(pipeIn.Close())
	check(<-done)
	bar.Finish()
	body, _ := ioutil.ReadAll(resp.Body)
	meta := Meta{}
	err = json.Unmarshal(body, &meta)
	if err != nil {
		fmt.Println(err, string(body))
	} else {
		fmt.Printf("%+v\n", meta) 
	}
}

func check(err error) {
	_, file, line, _ := runtime.Caller(1)
	if err != nil {
		log.Fatalf("Fatal from <%s:%d>\nError:%s", file, line, err)
	}
}

func (c *Client) upsync(localPath, parent string) {
	fmt.Printf("creating folder %q in %q\n", ospath.Base(localPath), parent)
	parentPath := c.mkfolder(ospath.Base(localPath), parent)
	if parentPath == "409 Conflict" {		
		parentPath = pth.Join(parent, ospath.Base(localPath))
		fmt.Println("conflict: folder %q already exists\n", parentPath) 
	}
	dirlist, err := ioutil.ReadDir(localPath)
	if err != nil {panic(err)}
	for _, f := range dirlist {
		if !f.IsDir() && strings.ToLower(f.Name()) != "thumbs.db" {
			filepath := ospath.Join(localPath, f.Name())
			fmt.Printf("uploading %q to %q\n", filepath, parentPath)
			c.pipedUpload(filepath, parentPath)
		}
		if f.IsDir() {
			folderPath := ospath.Join(localPath, f.Name())
			c.upsync(folderPath, parentPath)
		}
	}
}

func (c *Client) Upload(localPath, parent string) {
	stat, er := os.Stat(localPath)
	if er != nil {
		fmt.Println(er)
		os.Exit(2)
	}
	if !stat.IsDir(){
		c.pipedUpload(localPath, parent)
	} else {
		c.upsync(localPath, parent)
	}
}

func makeChunk(fh *os.File, offset int64) []byte {
	p := make([]byte, Chunksize)
	//fmt.Println("offset:", offset)
	n, _ := fh.ReadAt(p, offset)
	return p[:n]
}

func (c *Client) startUploadSession(fh *os.File) (cursor Cursor){
    //Returns json {"session_id": <session_id>}    

    uri, _ := url.Parse(content_url + "/files/upload_session/start")
    p := map[string]bool{"close": false}
	params, _ := json.Marshal(p)
	chunk := makeChunk(fh, 0)
	data := bytes.NewReader(chunk)
	req, err := http.NewRequest("POST", uri.String(), data)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(2) 
	}
	req.Header.Add("Authorization", "Bearer "+c.Auth.Token)
    req.Header.Add("Content-Type", "application/octet-stream")
    req.Header.Add("Dropbox-API-Arg", string(params))
    
    resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(2) 
	}
    defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
    if resp.StatusCode != 200 {
		fmt.Println("error: bad server status:", resp.Status)
		fmt.Println(string(body))		
	}
	err = json.Unmarshal(body, &cursor)
	if err != nil {
		fmt.Println(err)
	}
    return
}

func (c *Client) uploadSessionAppend(fh *os.File, cursor Cursor){
    //No return values. 
    uri, _ := url.Parse(content_url + "/files/upload_session/append_v2")
	offset := cursor.Offset
	chunk := makeChunk(fh, offset)
	data := bytes.NewReader(chunk)
	req, err := http.NewRequest("POST", uri.String(), data)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(2) 
	}
	req.Header.Add("Authorization", "Bearer "+c.Auth.Token)
    req.Header.Add("Content-Type", "application/octet-stream")
	type Params struct {
			Cursor Cursor `json:"cursor"`
			Close bool    `json:"close"`
	}
    p := Params{}
	p.Cursor = cursor 
	p.Close = false 
	params, _ := json.Marshal(p)
	//params map[string]Cursor{"cursor": cursor, "close": False}
    req.Header.Add("Dropbox-API-Arg", string(params)) 
    resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(2) 
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
    if resp.StatusCode != 200 {
		fmt.Println("error: bad server status:", resp.Status)
		fmt.Println("response body:", string(body))		
	}
}

func (c *Client) uploadSessionFinish(fh *os.File, cursor Cursor, remote_path string) (res Meta) {
    //Returns file props.
    uri, _ := url.Parse(content_url + "/files/upload_session/finish")
	type Commit struct {
			Path     string `json:"path"`
			Mode     string `json:"mode"`
			Autorename bool `json:"autorename"`
			Mute       bool `json:"mute"`
	}
	type Params struct {
			Cursor Cursor `json:"cursor"`
			Commit Commit `json:"commit"`
	}
	//params = map[string]{"cursor": cursor, "commit": {"path": remote_path, "mode": "add", "autorename": true, "mute": false}
	p := Params{}
	p.Cursor = cursor
	commit := Commit{Path: remote_path, Mode: "add", Autorename: true, Mute: false}
	p.Commit = commit
	params, _ := json.Marshal(p)
	offset := cursor.Offset
	chunk := makeChunk(fh, offset)
	data := bytes.NewReader(chunk)
	req, err := http.NewRequest("POST", uri.String(), data)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(2) 
	}
	req.Header.Add("Authorization", "Bearer "+c.Auth.Token)
    req.Header.Add("Dropbox-API-Arg", string(params)) 
    req.Header.Add("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("request: %+v\n", req)
		os.Exit(2) 
	}
    defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
    if resp.StatusCode != 200 {
		fmt.Println("error: bad server status:", resp.Status)
		fmt.Println("response body:", string(body))		
	}
	err = json.Unmarshal(body, &res)
	if err != nil {fmt.Println(err)}
	return
}

func (c *Client) ChunkedUpload(localPath, parent string) {
	stat, err := os.Stat(localPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	filesize := stat.Size()
	fmt.Printf("filesize: %d bytes\n", filesize)	
	fh, er := os.Open(localPath) 
	if er != nil {fmt.Println(er); os.Exit(2)}	
	if filesize <= Chunksize {
		c.pipedUpload(localPath, parent)
	} else {
		remotePath := pth.Join(parent, ospath.Base(fh.Name()))
		if remotePath[:1] != "/" {
			remotePath = "/" + remotePath
		}
        fmt.Println("starting upload session")
		fmt.Println("remote path:", remotePath)
        cursor := c.startUploadSession(fh)
        cursor.Offset += Chunksize
		position := Chunksize
        fmt.Printf("cursor: %+v\n", cursor)
        for position < filesize {
            if ((filesize - position) <= Chunksize){
                fmt.Println("Last chunk, finishing upload session...")
                finish := c.uploadSessionFinish(fh, cursor, remotePath)
                fmt.Printf("%+v\n", finish)
				position += Chunksize
            } else {
                fmt.Println("appending data; offset:", cursor.Offset)
                c.uploadSessionAppend(fh, cursor)
				position += Chunksize
                cursor.Offset = position			
			}
		}
	}
}

type Search struct {
		Matches []Match `json:"matches"`
		More bool `json:"more"`
		Start int `json:"start"`
}

type Match struct {
		MatchType struct {
			Tag string `json:".tag"`
		} `json:"match_type"`
		Metadata Meta `json:"metadata"`		
}

type Matchset []Match 
func(m Matchset) SetUser(user string, i int) {
	m[i].Metadata.User = user 
}

func (c *Client) doSearch(path, query string) Search {
	var result Search	
	ep := "/files/search"
	params := map[string]string{"path": path, "query":query}
	status, body := c.apiRequest("POST", ep, nil, params, true)
	if status == "200 OK" {
		err := json.Unmarshal(body, &result)
		if err != nil { fmt.Println(err);os.Exit(1) }		
	}	
	return result
}


func (c *Client) SearchUser(path, query string) {
	result := c.doSearch(path, query)
	matches := result.Matches
	count := len(matches)
	if path == "" {
		path = "/"
	} else if path [:1] != "/" { path = "/"+path}
	fmt.Printf("found %d item(s) in %s:\n\n", count, path)
	for _,e := range matches {
		if e.Metadata.Tag != "folder" {
			size, _ := utils.NiceBytes(e.Metadata.Size)
			fmt.Printf("%-70s %8s\n", e.Metadata.Name, size)
		} else {
			fmt.Println(e.Metadata.Name)
		}
	}
}

func (c *Client) SearchAll(path, query string){
	var res []Match	
	for user, _ := range users {
		var result Matchset
		c.SetToken(user)
		resp := c.doSearch(path, query)
		result = resp.Matches
		for k, _ := range result {
			result.SetUser(user, k)
		}
		res = append(result, res...)
	}	
	//sort.Sort(ByName(res))
	if path == "" {
		path = "/"
	} else if path [:1] != "/" { path = "/"+path}
	fmt.Printf("found %d items in %v:\n\n", len(res), path)
	for _, e := range res {
		if e.Metadata.Tag != "folder" {
			size, _ := utils.NiceBytes(e.Metadata.Size)
			fmt.Printf("[%s] %-65s %8s\n", users[e.Metadata.User], e.Metadata.PathDisplay, size)
		} else {
			fmt.Printf("[%s] %s\n", users[e.Metadata.User], e.Metadata.PathDisplay)
		}
	}
	fmt.Printf("\n[%s]\n", Legend(users))			       
}

func (c *Client) mkfolder(foldername, parent string) (p string) {
	path := pth.Join(parent, foldername)
	if path[:1] != "/" {
        path = "/" + path
	}
	form := map[string]string{"path": path}
	var res FolderMeta
	status, body := c.apiRequest("POST", "/files/create_folder_v2", nil, form, true)
	if status != "200 OK" {
		fmt.Println("error: bad status:", status)
		if status != "409 Conflict" {
			return
		}
	}
	err := json.Unmarshal(body, &res)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%+v\n", res)
	if status == "409 Conflict" {
		return status
	}
	return res.Metadata.PathDisplay
}

func (c *Client) CreateFolder(foldername, parent string) {
	c.mkfolder(foldername, parent)
}

func (c *Client) Move(src, dest string) {  
	type Resp struct {
			Metadata Meta `json:"metadata"` 
	}	
    if src[:1] != "/"{
		src = "/" + src
	}
    form := map[string]string{"from_path": src, "to_path": dest}
	status, body := c.apiRequest("POST", "/files/move_v2", nil, form, true)
	if	status != "200 OK" { fmt.Println("bad status:", status); os.Exit(1)}
	var res Resp
    err := json.Unmarshal(body, &res)
    if err != nil {fmt.Println(err); os.Exit(1)}
	if res.Metadata.Tag != "folder" {
		size, _ := utils.NiceBytes(res.Metadata.Size)
		fmt.Printf("path:%s size:%s\n", res.Metadata.PathDisplay, size)
	} else {
		fmt.Println("path:", res.Metadata.PathDisplay)
	}
}

func (c *Client) Remove(path string) {
	type Resp struct {
			Metadata Meta `json:"metadata"` 
	}	
    if path[:1] != "/" {
        path = "/" + path
	}
	params := map[string]string{"path":path}
    status, body := c.apiRequest("POST", "/files/delete_v2", nil, params, true)
	if	status != "200 OK" { fmt.Println("bad status:", status); os.Exit(1)}
	var res Resp
	err := json.Unmarshal(body, &res)
	if err != nil {panic(err)}
	fmt.Printf("%+v\n", res.Metadata)
}
