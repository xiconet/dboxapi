// Command dbox is a client for the dropbox "rest" API
package main

import(
	"os"
	"fmt"
	"strings"
	"github.com/xiconet/utils"
	"github.com/codegangsta/cli"
	pth "path"
	dbx "github.com/xiconet/dbox/dboxlib"
)

const(
	cfg_file = "C:/Users/ARC/.config/jagoodrop.yml"
	api_url = "https://api.dropbox.com/2"
)

var(
	users = map[string]string{"xiconet": "0", "jcoxi": "1", "jcoxinet": "2", "coxinet": "3"}
	chunksize = int64(8*1024*1024)
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


func main() {
	userlist := Userlist()
	uids := Uids()
	app := cli.NewApp()
	app.Name = "dropbox"
	app.Version = "0.50"
	app.Usage = "Client for the Dropbox API v2"
	
	app.Flags = []cli.Flag {
	cli.StringFlag{
		Name: "user, u",
		Value: "current_user",
		Usage: fmt.Sprintf("user name, one of %s", strings.Join(userlist, ", ")),
		},
	cli.BoolFlag{
		Name: "info, i",
		Usage: "get account info for the current/specified user",
		},
	cli.BoolFlag{
		Name: "meta, M",
		Usage: "get metadata for the specified path",
		},
	cli.BoolFlag{
		Name: "tree, t",
		Usage: "recursively list the specified path",
		},
	cli.BoolFlag{
		Name: "all_users, a",
		Usage: "list the specified path for all users",
		},
	cli.BoolFlag{
		Name: "link, k",
		Usage: "get streamable link(s) for item(s) under the specified path",
		},
	cli.BoolFlag{
		Name: "play, S",
		Usage: "stream link(s) for item(s) under the specified path in foobar2000 or VLC",
		},
	cli.BoolFlag{
		Name: "download, d",
		Usage: "download file(s) under the specified path",
		},
	cli.IntFlag{
		Name: "depth, r",
		Value: 1,
		Usage: "recursion depth for folder downloading",
		},
	cli.BoolFlag{
		Name: "aria, x",
		Usage: "use external (and faster) aria2c downloader",
		},
	cli.BoolFlag{
		Name: "fast, f",
		Usage: "download using parallel connections (internal code)",
		},
	cli.IntFlag{
		Name: "conns, c",
		Value: 5,
		Usage: "number of connections for 'fast' (parallel) downloading",
		},
	cli.IntFlag{
		Name: "parallel, P",
		Value: 0,
		Usage: "use with --d to download a folder's files by batches of <n> parallel goroutines",
		},
	cli.StringFlag{
		Name: "mkfolder, m",
		Value: "",
		Usage: "create a new folder in the specified parent folder path",
		},
	cli.StringFlag{
		Name: "upload, p",
		Value: "",
		Usage: "upload file(s) to the specified parent folder path",
		},
	cli.StringFlag{
		Name: "chunked_upload, cu",
		Value: "",
		Usage: "upload large file(s) by chunks to the specified parent folder path",
		},
	cli.IntFlag{
		Name: "chunk_size, cs",
		Value: 0,
		Usage: "chunk size in MiB to the specified parent folder path",
		},
	cli.StringFlag{
		Name: "move, mv",
		Value: "",
		Usage: "move and/or rename item(s) from <src> to <dest> full path",
		},
	cli.StringFlag{
		Name: "search, s",
		Value: "",
		Usage: "search for the specified <query> string",
		},
	cli.BoolFlag{
		Name: "remove, rm",
		Usage: "remove item(s) at the specified path",
		},
	}
	app.Action = func(c *cli.Context) {
		user := c.String("user")
		path := ""
		if len(c.Args()) > 0 {
			path = c.Args()[0]
		}		
		if user != "current_user" {
			if _, ok := users[user]; !ok {
				fmt.Printf("error: %q is not a registered user\n", user)
				fmt.Println("use one of", strings.Join(userlist, ", "))
				os.Exit(2)
			}
		}		
		if utils.StringInSlice(strings.Split(path, "/")[0], userlist) {
			user = strings.Split(path, "/")[0]
			path = strings.Join(strings.Split(path, "/")[1:], "/")
		} else if utils.StringInSlice(strings.Split(path, "/")[0], uids) {
			var err error
			user, err = UidToUser(strings.Split(path, "/")[0])
			if err != nil {
				fmt.Println(err); os.Exit(1)
			}
			path = strings.Join(strings.Split(path, "/")[1:], "/")			
		}
		if path != "" && path[:1] != "/" {path = "/" + path}
				
		d := dbx.NewClient(api_url, cfg_file, "", dbx.Auth{}, map[string]string{})
		if !c.Bool("all") { d.SetToken(user) }
		switch {
		case c.Bool("tree"):
			depth := c.Int("depth")
			if depth == 1 {depth = 0}
			d.GetTree(path, depth, 0)
		case c.Bool("info"):
			d.Info()
		case c.Bool("download"):
			localPath := pth.Base(path)
			d.Download(path, localPath, c.Bool("aria"), c.Bool("fast"), c.Int("depth"), c.Int("parallel"), c.Int("conns"))
		case c.Bool("link"):
			stream := false
			d.GetLinks(path, stream) 
		case c.Bool("play"):
			d.StreamLinks(path)
		case c.String("search") != "" :
			query := c.String("search")
			if c.Bool("all_users") {
				d.SearchAll(path, query)
			} else {
				d.SearchUser(path, query)
			}
		case c.String("move") != "" :
			d.Move(c.String("move"), path)
		case c.String("upload") != "" :
			d.Upload(c.String("upload"), path)
		case c.String("chunked_upload") != "" :
			if c.Int("chunk_size") > 0 {
				dbx.Chunksize = int64(c.Int("chunk_size")*1024*1024)
			}
			d.ChunkedUpload(c.String("chunked_upload"), path)
		case c.String("mkfolder") != "" :
			d.CreateFolder(c.String("mkfolder"), path)
		case c.Bool("remove"):
			if path == "/" {
				fmt.Println("error: cannot remove root folder")
				os.Exit(2)			
			} 
			d.Remove(path)
		case c.Bool("meta"):
			meta, _ := d.GetMetadata(path)
			fmt.Printf("%+v\n", meta)
		default:
			if c.Bool("all_users") {
				d.ListAll(path)
			} else {
				d.ListFolder(path)
			}
		}
	}
  app.Run(os.Args)
}		