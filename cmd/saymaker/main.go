// Command saymaker generates an image or a video clip on SayMaker
// (https://saymaker.ai) and prints the URL of the finished file.
//
//	saymaker "a paper boat at sunrise"
//	saymaker -video -duration 6 "a fox trotting through fresh snow"
//	saymaker -model gpt-image-2-5-flare -aspect 16:9 "isometric diorama of a ramen shop"
//	saymaker -image https://example.com/p.jpg "change the jacket to dark green, keep the face"
//
// Set SAYMAKER_API_KEY (create a key at https://saymaker.ai/settings/apikeys).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hanshs474/saymaker-go"
)

var version = "0.1.0"

func main() {
	video := flag.Bool("video", false, "generate a video clip instead of an image")
	model := flag.String("model", "", "model id (e.g. gpt-image-2-5-flare, veo-3-1, kling-3-0, seedance-2)")
	image := flag.String("image", "", "public image URL: the photo to edit, or the first frame of a clip")
	aspect := flag.String("aspect", "", "aspect ratio, e.g. 1:1, 16:9, 9:16")
	duration := flag.Int("duration", 0, "clip length in seconds, where the model offers a choice")
	resolution := flag.String("resolution", "", "clip resolution, e.g. 480p, 720p, 1080p")
	showVersion := flag.Bool("version", false, "print the version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: saymaker [flags] \"prompt\"\n\nGenerates on https://saymaker.ai and prints the file URL. Needs SAYMAKER_API_KEY.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("saymaker", version)
		return
	}
	prompt := strings.Join(flag.Args(), " ")
	if prompt == "" {
		flag.Usage()
		os.Exit(2)
	}
	var url string
	var err error
	if *video {
		url, err = saymaker.GenerateVideo(context.Background(), prompt, saymaker.VideoOptions{
			Model: *model, ImageURL: *image, AspectRatio: *aspect, Duration: *duration, Resolution: *resolution,
		})
	} else {
		url, err = saymaker.Generate(context.Background(), prompt, saymaker.Options{
			Model: *model, ImageURL: *image, AspectRatio: *aspect,
		})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, saymaker.ErrAuth) {
			os.Exit(3)
		}
		os.Exit(1)
	}
	fmt.Println(url)
}
