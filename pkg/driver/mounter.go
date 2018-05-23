package driver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
)

// Methods to format and mount

const (
	multipathTimeout  = 10 * time.Second
	multipathExec     = "/sbin/multipath"
	multipathBindings = "/etc/multipath/bindings"
)

type Mounter struct{}

func packetVolumeIDToName(id string) string {
	// "3ee59355-a51a-42a8-b848-86626cc532f0" -> "volume-3ee59355"
	uuidElements := strings.Split(id, "-")
	return fmt.Sprintf("volume-%s", uuidElements[0])
}

func multipath(args ...string) (string, error) {

	ctx, cancel := context.WithTimeout(context.Background(), multipathTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, multipathExec, args...)

	output, err := cmd.Output()

	if ctx.Err() == context.DeadlineExceeded {
		glog.V(5).Infof("Multipath (%s) timed out after %v", strings.Join(args, " "), multipathTimeout)
		return string(output), nil
	}

	return string(output), err
}

func getScsiID(device string) (string, error) {
	devicePath := filepath.Join("/dev", device)
	out, err := exec.Command("/lib/udev/scsi_id", "-g", "-u", "-d", devicePath).Output()
	return string(out), err
}

func getDevice(portal, iqn string) (string, error) {

	pattern := fmt.Sprintf("%s*%s*%s*", "/dev/disk/by-path/", portal, iqn)

	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("file not found for pattern %s", pattern)
	}

	file := files[0]
	finfo, err := os.Lstat(file)
	if err != nil {
		return "", err
	}
	if finfo.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("file %s is not a link", file)
	}
	source, err := os.Readlink(file)
	if err != nil {
		return "", err
	}
	linkedFile, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	return linkedFile.Name(), nil

}

func iscsiadminDiscover(ip string) error {
	args := fmt.Sprintf("--mode discovery --portal %s --type sendtargets --discover", ip)
	_, err := exec.Command("iscsiadm", args).Output()
	if err != nil {
		return err
	}
	return nil
}

func iscsiadminLogin(ip, iqn string) error {
	args := fmt.Sprintf("--mode node --portal %s  --targetname %s --login", ip, iqn)
	_, err := exec.Command("iscsiadm", args).Output()
	if err != nil {
		return err
	}
	return nil
}

func iscsiadminLogout(ip, iqn string) error {
	args := fmt.Sprintf("--mode node --portal %s  --targetname %s --logout", ip, iqn)
	_, err := exec.Command("iscsiadm", args).Output()
	if err != nil {
		return err
	}
	return nil
}

func mountFs(src, target string) error {
	_, err := exec.Command("mount", "-t", "ext4", "--source", src, "--target", target).Output()
	return err
}

func unmountFs(path string) error {
	_, err := exec.Command("umount", path).Output()
	return err
}

func mountMappedDevice(device, target string) error {
	devicePath := filepath.Join("/dev/mapper/", device)
	return mountFs(devicePath, target)

}

// etx4 format
func formatMappedDevice(device string) error {
	devicePath := filepath.Join("/dev/mapper/", device)
	fstype := "etx4"
	_, err := exec.Command("mkfs."+fstype, "-F", devicePath).Output()
	return err
}

// represents the lsblk info
type blockInfo struct {
	Name       string `json:"name"`
	FsType     string `json:"fstype"`
	Label      string `json:"label"`
	UUID       string `json:"uuid"`
	Mountpoint string `json:"mountpoint"`
}

// represents the lsblk info
type deviceset struct {
	BlockDevices []blockInfo `json:"blockdevices"`
}

// get info
var execCommand = exec.Command

func getMappedDevice(device string) (blockInfo, error) {
	devicePath := filepath.Join("/dev/mapper/", device)

	// testing issue: must mock out call to Stat as well as to exec.Command
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		return blockInfo{}, err
	}

	// use -J json output so we can parse it into a blockInfo struct
	out, err := execCommand("lsblk", "-J", "-i", "--output", "NAME,FSTYPE,LABEL,UUID,MOUNTPOINT", devicePath).Output()
	if err != nil {
		return blockInfo{}, err
	}
	devices := deviceset{}
	err = json.Unmarshal(out, &devices)
	if err != nil {
		return blockInfo{}, err
	}
	for _, info := range devices.BlockDevices {
		if info.Name == device {
			return info, nil
		}
	}
	return blockInfo{}, fmt.Errorf("device %s not found", device)
}

// read the bindings from /etc/multipath/bindings
func readBindings() (map[string]string, map[string]string, error) {

	var bindings = map[string]string{}
	var discard = map[string]string{}
	f, err := os.Open(multipathBindings)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] != '#' {
			elements := strings.Fields(line)
			if len(elements) == 2 {

				if strings.HasPrefix(elements[0], "mpath") {
					discard[elements[0]] = elements[1]
				} else {
					bindings[elements[0]] = elements[1]
				}
			}
		}
	}

	return bindings, discard, nil
}

// read the bindings to /etc/multipath/bindings
func writeBindings(bindings map[string]string) error {

	f, err := os.Create(multipathBindings)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for name, id := range bindings {
		writer.WriteString(fmt.Sprintf("%s %s\n", name, id))
	}
	writer.Flush()
	return nil
}
