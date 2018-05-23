package driver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func getScsiID(devicePath string) (string, error) {
	args := []string{"-g", "-u", "-d", devicePath}
	out, err := exec.Command("/lib/udev/scsi_id", args...).Output()
	if err != nil {
		glog.V(5).Infof("/lib/udev/scsi_id %v : %s, %v", args, out, err)
		return "", err
	}
	return string(out), nil
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
	source, err := filepath.EvalSymlinks(file)
	if err != nil {
		glog.V(5).Infof("cannot get symlink for %s", file)
		return "", err
	}
	return source, nil
	// linkedFile, err := os.Stat(source)
	// if err != nil {
	// 	return "", err
	// }
	// return linkedFile.Name(), nil

}

func iscsiadminDiscover(ip string) error {
	args := []string{"--mode", "discovery", "--portal", ip, "--type", "sendtargets", "--discover"}
	output, err := exec.Command("iscsiadm", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("iscsiadm %v : %s, %v", args, output, err)
		return err
	}
	return nil
}

func iscsiadminHasSession(ip, iqn string) (bool, error) {
	args := []string{"--mode", "session"}
	out, err := exec.Command("iscsiadm", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("iscsiadm %v : %s, %v", args, out, err)
		return false, nil // this is almost certainly "No active sessions"
	}
	pat, err := regexp.Compile(ip + ".*" + iqn)
	if err != nil {
		return false, err
	}
	// glog.V(5).Infof("iscsiadm sessions %s", string(out[:]))
	lines := strings.Split(string(out[:]), "\n")
	for _, line := range lines {
		found := pat.FindString(line)

		if found != "" {
			return true, nil
		}
		// glog.V(5).Infof("iscsiadm session not found in line: %s", line)
	}
	return false, nil
}

func iscsiadminLogin(ip, iqn string) error {
	hasSession, err := iscsiadminHasSession(ip, iqn)
	if err != nil {
		return err
	}
	if hasSession {
		return nil
	}
	args := []string{"--mode", "node", "--portal", ip, "--targetname", iqn, "--login"}
	output, err := exec.Command("iscsiadm", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("iscsiadm %v : %s, %v", args, output, err)
		return err
	}
	return nil
}

func iscsiadminLogout(ip, iqn string) error {
	hasSession, err := iscsiadminHasSession(ip, iqn)
	if err != nil {
		return err
	}
	if !hasSession {
		return nil
	}
	args := []string{"--mode", "node", "--portal", ip, "--targetname", iqn, "--logout"}
	out, err := exec.Command("iscsiadm", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("iscsiadm %v : %s, %v", args, out, err)
		return err
	}
	return nil
}

func bindmountFs(src, target string) error {

	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(target, 0755)
		} else {
			glog.V(5).Infof("stat %s, %v", target, err)
			return err
		}
	}
	_, err := os.Stat(target)
	if err != nil {
		glog.V(5).Infof("stat %s, %v", target, err)
		return err
	}
	args := []string{"--bind", src, target}
	out, err := exec.Command("mount", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("mount %v : %s, %v", args, out, err)
		return err
	}
	return nil
}

func unmountFs(path string) error {
	args := []string{path}
	out, err := exec.Command("umount", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("umount %v : %s, %v", args, out, err)
		return err
	}
	return nil
}

func mountMappedDevice(device, target string) error {
	devicePath := filepath.Join("/dev/mapper/", device)
	args := []string{"-t", "ext4", "--source", devicePath, "--target", target}
	out, err := exec.Command("mount", args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("mount %v : %s, %v", args, out, err)
		return err
	}
	return nil

}

// etx4 format
func formatMappedDevice(device string) error {
	devicePath := filepath.Join("/dev/mapper/", device)
	args := []string{"-F", devicePath}
	fstype := "ext4"
	command := "mkfs." + fstype
	out, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("%s %v : %s, %v", command, args, out, err)
		return err
	}
	return nil
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

	if _, err := os.Stat(multipathBindings); err != nil {
		if os.IsNotExist(err) {
			// file does not exist
			return bindings, discard, nil
		} else {
			return nil, nil, err
		}
	}

	f, err := os.Open(multipathBindings)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && line[0] != '#' {
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
