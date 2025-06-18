package main

import "os"
import "os/exec"

func main() {
    exec.Command("go", append([]string{"run", "cloud-installer.go"}, os.Args[1:]...)...).Run()
}
