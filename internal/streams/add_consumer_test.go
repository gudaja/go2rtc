package streams

import (
	"testing"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/stretchr/testify/require"
)

type testProducer struct {
	core.Connection
}

func (p *testProducer) GetTrack(media *core.Media, codec *core.Codec) (*core.Receiver, error) {
	track := core.NewReceiver(media, codec)
	p.Receivers = append(p.Receivers, track)
	return track, nil
}

func (p *testProducer) Start() error { return nil }

type testConsumer struct {
	core.Connection
}

func (c *testConsumer) AddTrack(media *core.Media, codec *core.Codec, track *core.Receiver) error {
	sender := core.NewSender(media, codec)
	sender.Bind(track)
	c.Senders = append(c.Senders, sender)
	return nil
}

func newTestStream(t *testing.T, scheme string, medias ...*core.Media) *Stream {
	HandleFunc(scheme, func(string) (core.Producer, error) {
		return &testProducer{Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: scheme,
			Medias:     medias,
		}}, nil
	})
	stream, err := New(scheme, scheme+":camera")
	require.NoError(t, err)
	return stream
}

func media(kind, direction string, codecs ...string) *core.Media {
	m := &core.Media{Kind: kind, Direction: direction}
	for _, name := range codecs {
		codec := &core.Codec{Name: name, ClockRate: 90000}
		if kind == core.KindAudio {
			codec.ClockRate = 8000
		}
		m.Codecs = append(m.Codecs, codec)
	}
	return m
}

// webrtcClient mimics the app offer: H264-only video, opus/PCMU/PCMA audio
func webrtcClient() *testConsumer {
	return &testConsumer{Connection: core.Connection{Medias: []*core.Media{
		media(core.KindVideo, core.DirectionSendonly, core.CodecH264),
		media(core.KindAudio, core.DirectionSendonly, core.CodecPCMU, core.CodecPCMA),
	}}}
}

// Camera on 192.168.1.144/stream0: H265 video + PCMU audio. The H264-only
// client used to get an audio-only session with no error at all.
func TestAddConsumerVideoNotMatched(t *testing.T) {
	stream := newTestStream(t, "h265cam",
		media(core.KindVideo, core.DirectionRecvonly, core.CodecH265),
		media(core.KindAudio, core.DirectionRecvonly, core.CodecPCMU),
	)

	cons := webrtcClient()
	err := stream.AddConsumer(cons)
	require.EqualError(t, err, "streams: codecs not matched: video:H265, audio:PCMU => video:H264, audio:PCMU, audio:PCMA")
	require.Empty(t, stream.consumers)

	// audio track was bound and must be released, so the producer can stop
	for _, track := range stream.producers[0].receivers {
		require.Empty(t, track.Senders())
	}
}

// Unmatched audio is optional - video only session is fine.
func TestAddConsumerAudioNotMatched(t *testing.T) {
	stream := newTestStream(t, "aaccam",
		media(core.KindVideo, core.DirectionRecvonly, core.CodecH264),
		media(core.KindAudio, core.DirectionRecvonly, core.CodecAAC),
	)

	require.NoError(t, stream.AddConsumer(webrtcClient()))
	require.Len(t, stream.consumers, 1)
}

// Camera without video (ex. audio only source) is not an error.
func TestAddConsumerNoVideoInSource(t *testing.T) {
	stream := newTestStream(t, "audiocam",
		media(core.KindAudio, core.DirectionRecvonly, core.CodecPCMU),
	)

	require.NoError(t, stream.AddConsumer(webrtcClient()))
	require.Len(t, stream.consumers, 1)
}
