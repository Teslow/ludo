package scanner

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/libretro/ludo/notifications"
	"github.com/libretro/ludo/playlists"
	"github.com/libretro/ludo/rdb"
	"github.com/libretro/ludo/state"
	"github.com/libretro/ludo/utils"
)

// LoadDB loops over the RDBs in a given directory and parses them
func LoadDB(dir string) (rdb.DB, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return rdb.DB{}, err
	}
	db := make(rdb.DB)
	for _, f := range files {
		if !strings.Contains(f.Name(), ".rdb") {
			continue
		}
		filename := f.Name()
		system := filename[0 : len(filename)-4]
		bytes, _ := ioutil.ReadFile(dir + f.Name())
		db[system] = rdb.Parse(bytes)
	}
	return db, nil
}

// ScanDir scans a full directory, report progress and generate playlists
func ScanDir(dir string) {
	usr, _ := user.Current()
	roms := utils.AllFilesIn(dir)
	scannedGames := make(chan (rdb.Entry))
	go Scan(dir, roms, scannedGames, state.Global.DB.Find)
	go func() {
		i := 0
		for game := range scannedGames {
			if playlists.ExistsInPlaylist(game.System, game.Path, game.CRC32) {
				continue
			}
			i++
			lpl, _ := os.OpenFile(usr.HomeDir+"/.ludo/playlists/"+game.System+".lpl", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			lpl.WriteString(game.Path + "\n")
			lpl.WriteString(game.Name + "\n")
			lpl.WriteString("DETECT\n")
			lpl.WriteString("DETECT\n")
			lpl.WriteString(strconv.FormatUint(uint64(game.CRC32), 10) + "|crc\n")
			lpl.WriteString(game.System + ".lpl\n")
			lpl.Close()
		}
	}()
}

// Scan scans a list of roms against the database
func Scan(dir string, roms []string, games chan (rdb.Entry), cb func(rompath string, romname string, CRC32 uint32, games chan (rdb.Entry))) {
	nid := notifications.DisplayAndLog("Menu", "Scanning %s", dir)
	for i, f := range roms {
		ext := filepath.Ext(f)
		switch ext {
		case ".zip":
			// Open the ZIP archive
			z, _ := zip.OpenReader(f)
			for _, rom := range z.File {
				if rom.CRC32 > 0 {
					// Look for a matching game entry in the database
					cb(f, rom.Name, rom.CRC32, games)
					notifications.Update(nid, strconv.Itoa(i)+"/"+strconv.Itoa(len(roms))+" "+f)
				}
			}
			z.Close()
		}
	}
	notifications.Update(nid, "Done scanning.")
}
