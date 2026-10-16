package streams

import (
	"net/url"
	"strings"
	"testing"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/stretchr/testify/require"
)

// profilesHandler mimics a camera with H265 main and H264 sub profile
func profilesHandler(scheme string) {
	HandleFunc(scheme, func(source string) (core.Producer, error) {
		video := core.CodecH264
		if strings.HasSuffix(source, "/stream0") {
			video = core.CodecH265
		}
		return &testProducer{Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: scheme,
			Medias: []*core.Media{
				media(core.KindVideo, core.DirectionRecvonly, video),
				media(core.KindAudio, core.DirectionRecvonly, core.CodecPCMU),
			},
		}}, nil
	})
}

func producerURLs(stream *Stream) (urls []string) {
	for _, prod := range stream.producers {
		urls = append(urls, prod.url)
	}
	return
}

func getOrPatch(t *testing.T, rawQuery string) *Stream {
	query, err := url.ParseQuery(rawQuery)
	require.NoError(t, err)
	stream, err := GetOrPatch(query)
	require.NoError(t, err)
	return stream
}

// The app sends camera url and url2 as src and src2.
func TestGetOrPatchSrc2(t *testing.T) {
	profilesHandler("src2cam")

	stream := getOrPatch(t, "src=src2cam://cam/stream0&src2=src2cam://cam/stream1")
	require.Equal(t, []string{"src2cam://cam/stream0", "src2cam://cam/stream1"}, producerURLs(stream))

	// H264-only client gets video from the second profile
	require.NoError(t, stream.AddConsumer(webrtcClient()))
	require.Len(t, stream.consumers, 1)

	// and audio from the same profile, the first one is not opened at all
	require.Empty(t, stream.producers[0].receivers)
	require.Len(t, stream.producers[1].receivers, 2)

	// same request returns the same stream
	require.Equal(t, stream, getOrPatch(t, "src=src2cam://cam/stream0&src2=src2cam://cam/stream1"))
}

func TestGetOrPatchSrcNumbered(t *testing.T) {
	profilesHandler("src3cam")

	// empty url3 is skipped
	stream := getOrPatch(t, "src=src3cam://cam/stream0&src2=src3cam://cam/stream1&src3=")
	require.Equal(t, []string{"src3cam://cam/stream0", "src3cam://cam/stream1"}, producerURLs(stream))

	// repeated src still works
	stream = getOrPatch(t, "src=src3cam://cam/a&src=src3cam://cam/b&src2=src3cam://cam/c")
	require.Equal(t, []string{"src3cam://cam/a", "src3cam://cam/b", "src3cam://cam/c"}, producerURLs(stream))

	// empty src2 means a single-source stream
	stream = getOrPatch(t, "src=src3cam://cam/single&src2=")
	require.Equal(t, []string{"src3cam://cam/single"}, producerURLs(stream))
}

// A single-source stream from the first profile (ex. an earlier request
// with url only) must not be reused for a multi-source request.
func TestGetOrPatchMultiAfterSingle(t *testing.T) {
	profilesHandler("seqcam")

	single := getOrPatch(t, "src=seqcam://cam/stream0")
	require.Error(t, single.AddConsumer(webrtcClient()))

	multi := getOrPatch(t, "src=seqcam://cam/stream0&src2=seqcam://cam/stream1")
	require.NotEqual(t, single, multi)
	require.Equal(t, []string{"seqcam://cam/stream0", "seqcam://cam/stream1"}, producerURLs(multi))
	require.NoError(t, multi.AddConsumer(webrtcClient()))
}
