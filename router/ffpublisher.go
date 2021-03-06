package router

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"

	"github.com/notedit/sdp"

	mediaserver "github.com/notedit/media-server-go"
)

/**
ffmpeg -fflags nobuffer -i rtmp://ali.wangxiao.eaydu.com/live_bak/x_100_rtc_test \
-vcodec copy -an -bsf:v h264_mp4toannexb,dump_extra -f rtp -payload_type 96 rtp://127.0.0.1:5000 \
-acodec libopus -vn -ar 48000 -ac 2 -f rtp -payload_type 96 rtp://127.0.0.1:5002
*/

// FFPublisher publisher
type FFPublisher struct {
	id           string
	streamURL    string
	command      *exec.Cmd
	stdStdinPipe io.WriteCloser
	videoSession *mediaserver.StreamerSession
	audioSession *mediaserver.StreamerSession
	capabilities map[string]*sdp.Capability
}

// NewFFPublisher  new ffmpeg publisher
func NewFFPublisher(streamID string, streamURL string, capabilities map[string]*sdp.Capability) *FFPublisher {

	publisher := &FFPublisher{}
	publisher.id = streamID
	publisher.capabilities = capabilities
	publisher.streamURL = streamURL

	return publisher
}

// Start start the pipeline
func (p *FFPublisher) Start() <-chan error {

	done := make(chan error, 1)

	videoMediaInfo := sdp.MediaInfoCreate("video", p.capabilities["video"])
	videoPt := videoMediaInfo.GetCodec("h264").GetType()
	p.videoSession = mediaserver.NewStreamerSession(videoMediaInfo)

	audioMediaInfo := sdp.MediaInfoCreate("audio", p.capabilities["audio"])
	audioPt := audioMediaInfo.GetCodec("opus").GetType()
	p.audioSession = mediaserver.NewStreamerSession(audioMediaInfo)

	command := []string{
		"-i", p.streamURL,
		"-fflags", "nobuffer",
		"-vcodec", "copy", "-an", "-bsf:v", "h264_mp4toannexb",
		"-f", "rtp",
		"-payload_type", strconv.Itoa(videoPt),
		"rtp://127.0.0.1:" + strconv.Itoa(p.videoSession.GetLocalPort()),
		"-acodec", "libopus",
		"-vn", "-ar", "48000", "-ac", "2",
		"-f", "rtp",
		"-payload_type", strconv.Itoa(audioPt),
		"rtp://127.0.0.1:" + strconv.Itoa(p.audioSession.GetLocalPort()),
	}

	p.command = exec.Command("ffmpeg", command...)

	out := &bytes.Buffer{}

	p.command.Stdout = out

	stdin, err := p.command.StdinPipe()
	if nil != err {
		fmt.Println("Stdin not available: " + err.Error())
	}

	p.stdStdinPipe = stdin

	err = p.command.Start()

	go func(err error, out *bytes.Buffer) {
		if err != nil {
			done <- fmt.Errorf("Failed Start FFMPEG with %s, message %s", err, out.String())
			close(done)
			return
		}
		err = p.command.Wait()
		if err != nil {
			err = fmt.Errorf("Failed Finish FFMPEG with %s, message %s", err, out.String())
		}
		done <- err
		close(done)
	}(err, out)

	return done
}

// GetID  get publisher id
func (p *FFPublisher) GetID() string {
	return p.id
}

// GetAnswer get answer str
func (p *FFPublisher) GetAnswer() string {
	return ""
}

// GetVideoTrack get video track
func (p *FFPublisher) GetVideoTrack() *mediaserver.IncomingStreamTrack {

	if p.videoSession != nil {
		return p.videoSession.GetIncomingStreamTrack()
	}
	return nil
}

// GetAudioTrack get audio track
func (p *FFPublisher) GetAudioTrack() *mediaserver.IncomingStreamTrack {

	if p.audioSession != nil {
		return p.audioSession.GetIncomingStreamTrack()
	}
	return nil
}

// Stop  stop this publisher
func (p *FFPublisher) Stop() {

	if p.audioSession != nil {
		p.audioSession.Stop()
	}

	if p.videoSession != nil {
		p.videoSession.Stop()
	}

	if p.command != nil {
		stdin := p.stdStdinPipe
		if stdin != nil {
			stdin.Write([]byte("q\n"))
		}
	}
}
