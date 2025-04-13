package roc

/*
#include <roc/sender_encoder.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
)

// Sender encoder.
//
// Sender encoder gets an audio stream from the user, encodes it into network packets, and
// provides encoded packets to the user.
//
// Sender encoder is a networkless single-stream version of \ref roc_sender. It implements
// the same pipeline, but instead of sending packets to network, it returns them to the
// user. The user is responsible for carrying packets over network. Unlike \ref
// roc_sender, it doesn't support multiple slots and connections. It produces traffic for
// a single remote peer.
//
// For detailed description of sender pipeline, see documentation for \ref roc_sender.
//
// # Life cycle
//
// - Encoder is created using OpenSenderEncoder().
//
//   - The user activates one or more interfaces by invoking SenderEncoder.Activate().
//     This tells encoder what types of streams to produces and what protocols to use for
//     them (e.g. only audio packets or also redundancy packets).
//
//   - The audio stream is iteratively pushed to the encoder using
//     SenderEncoder.PushFrame(). The sender encodes the stream into packets and
//     accumulates them in internal queue.
//
//   - The packet stream is iteratively popped from the encoder internal queue using
//     SenderEncoder.PopPacket(). User should retrieve all available packets from all
//     activated interfaces every time after pushing a frame.
//
//   - User is responsible for delivering packets to \ref roc_receiver_decoder and pushing
//     them to appropriate interfaces of decoder.
//
//   - In addition, if a control interface is activated, the stream of encoded feedback
//     packets from decoder is pushed to encoder internal queue using
//     SenderEncoder.PushFeedbackPacket().
//
//   - User is responsible for delivering feedback packets from \ref roc_receiver_decoder
//     and pushing them to appropriate interfaces of encoder.
//
// - Encoder is eventually destroyed using SenderEncoder.Close().
//
// # Interfaces and protocols
//
// Sender encoder may have one or several //interfaces//, as defined in \ref roc_interface.
// The interface defines the type of the communication with the remote peer and the set
// of the protocols supported by it.
//
// Each interface has its own outbound packet queue. When a frame is pushed to the
// encoder, it may produce multiple packets for each interface queue. The user then should
// pop packets from each interface that was activated.
//
// # Feedback packets
//
// Control interface in addition has inbound packet queue. The user should push feedback
// packets from decoder to this queue. When a frame is pushed to encoder, it consumes
// those accumulated packets.
//
// The user should deliver feedback packets from decoder back to encoder. Feedback packets
// allow decoder and encoder to exchange metrics like latency and losses, and several
// features like latency calculations require feedback to function properly.
//
// # Thread safety
//
// Can be used concurrently.
type SenderEncoder struct {
	mu   sync.RWMutex
	cPtr *C.roc_sender_encoder
}

// Opens a new SenderEncoder
//
// Allocates a new Encoder and attaches to the context
func OpenSenderEncoder(context *Context, config SenderConfig) (senderEncoder *SenderEncoder, err error) {
	logWrite(LogDebug, "entering OpenSenderEncoder(): context=%p config=%+v", context, config)
	defer func() {
		logWrite(LogDebug,
			"leaving OpenSenderEncoder(): context=%p sender=%p err=%#v", context, senderEncoder, err,
		)
	}()

	checkVersionFn()

	if context == nil {
		return nil, errors.New("context is nil")
	}

	context.mu.RLock()
	defer context.mu.RUnlock()

	if context.cPtr == nil {
		return nil, errors.New("context is closed")
	}

	cPacketLength, err := go2cUnsignedDuration(config.PacketLength)
	if err != nil {
		return nil, fmt.Errorf("invalid config.PacketLength: %w", err)
	}

	cTargetLatency, err := go2cUnsignedDuration(config.TargetLatency)
	if err != nil {
		return nil, fmt.Errorf("invalid config.TargetLatency: %w", err)
	}

	cLatencyTolerance, err := go2cUnsignedDuration(config.LatencyTolerance)
	if err != nil {
		return nil, fmt.Errorf("invalid config.LatencyTolerance: %w", err)
	}

	cConfig := C.struct_roc_sender_config{
		frame_encoding: C.struct_roc_media_encoding{
			rate:     C.uint(config.FrameEncoding.Rate),
			format:   C.roc_format(config.FrameEncoding.Format),
			channels: C.roc_channel_layout(config.FrameEncoding.Channels),
			tracks:   C.uint(config.FrameEncoding.Tracks),
		},
		packet_encoding:          C.roc_packet_encoding(config.PacketEncoding),
		packet_length:            cPacketLength,
		packet_interleaving:      go2cBool(config.PacketInterleaving),
		fec_encoding:             C.roc_fec_encoding(config.FecEncoding),
		fec_block_source_packets: C.uint(config.FecBlockSourcePackets),
		fec_block_repair_packets: C.uint(config.FecBlockRepairPackets),
		clock_source:             C.roc_clock_source(config.ClockSource),
		latency_tuner_backend:    C.roc_latency_tuner_backend(config.LatencyTunerBackend),
		latency_tuner_profile:    C.roc_latency_tuner_profile(config.LatencyTunerProfile),
		resampler_backend:        C.roc_resampler_backend(config.ResamplerBackend),
		resampler_profile:        C.roc_resampler_profile(config.ResamplerProfile),
		target_latency:           cTargetLatency,
		latency_tolerance:        cLatencyTolerance,
	}

	var cSenderEncoder *C.roc_sender_encoder
	errCode := C.roc_sender_encoder_open(context.cPtr, &cConfig, &cSenderEncoder)
	if errCode != 0 {
		return nil, newNativeErr("roc_sender_encoder_open()", errCode)
	}
	if cSenderEncoder == nil {
		panic("roc_sender_encoder_open() returned nil")
	}

	senderEncoder = &SenderEncoder{
		cPtr: cSenderEncoder,
	}

	return senderEncoder, nil
}
