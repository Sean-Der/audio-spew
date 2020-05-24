package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
)

const pathToAudio = "opusSampleFrames"

func main() {
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/connect", connect)
	panic(http.ListenAndServe(":3000", nil))

}

func connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		panic(fmt.Sprintf("Expected 'POST' got '%v'", r.Method))
	}

	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		panic(err)
	}

	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.PopulateFromSDP(offer); err != nil {
		panic(err)
	}

	var payloadType uint8
	for _, codec := range mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeAudio) {
		if strings.EqualFold(codec.Name, "opus") {
			payloadType = codec.PayloadType
			break
		}
	}
	if payloadType == 0 {
		panic("Remote peer does not support VP8")
	}

	peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	track, err := peerConnection.NewTrack(payloadType, rand.Uint32(), "audio", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(track); err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	out, err := json.Marshal(answer)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, string(out))

	go func() {
		for {
			for _, file := range getFileList() {
				opusSample, err := ioutil.ReadFile(file)
				if err != nil {
					panic(err)
				}

				if err := track.WriteSample(media.Sample{Data: opusSample, Samples: media.NSamples(20*time.Millisecond, 48000)}); err != nil {
					panic(err)
				}
				time.Sleep(time.Millisecond * 20)
			}
		}
	}()
}

func getFileList() (files []string) {
	if err := filepath.Walk(pathToAudio, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".opus") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		panic(err)
	}
	return
}
