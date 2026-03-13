package midi

/*
#cgo LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

// alsaConn holds the sequencer handle and our local port.
typedef struct {
    snd_seq_t* seq;
    int        local_port;
    int        dev_client;
    int        dev_port;
} alsaConn;

static alsaConn* alsa_open(const char* client_name) {
    alsaConn* c = calloc(1, sizeof(alsaConn));
    if (!c) return NULL;
    if (snd_seq_open(&c->seq, "default", SND_SEQ_OPEN_DUPLEX, SND_SEQ_NONBLOCK) < 0) {
        free(c);
        return NULL;
    }
    snd_seq_set_client_name(c->seq, client_name);
    c->local_port = snd_seq_create_simple_port(c->seq, "nanoctl",
        SND_SEQ_PORT_CAP_READ   | SND_SEQ_PORT_CAP_SUBS_READ |
        SND_SEQ_PORT_CAP_WRITE  | SND_SEQ_PORT_CAP_SUBS_WRITE,
        SND_SEQ_PORT_TYPE_MIDI_GENERIC | SND_SEQ_PORT_TYPE_APPLICATION);
    if (c->local_port < 0) {
        snd_seq_close(c->seq);
        free(c);
        return NULL;
    }
    c->dev_client = -1;
    c->dev_port   = -1;
    return c;
}

static void alsa_close(alsaConn* c) {
    if (!c) return;
    snd_seq_close(c->seq);
    free(c);
}

// alsa_find_port scans all sequencer clients/ports for one whose client name
// or port name contains nameSubstr.  Fills c->dev_client / c->dev_port and
// subscribes for both sending and receiving.  Returns 0 on success.
static int alsa_find_port(alsaConn* c, const char* nameSubstr) {
    snd_seq_client_info_t* cinfo;
    snd_seq_port_info_t*   pinfo;
    snd_seq_client_info_alloca(&cinfo);
    snd_seq_port_info_alloca(&pinfo);

    snd_seq_client_info_set_client(cinfo, -1);
    while (snd_seq_query_next_client(c->seq, cinfo) >= 0) {
        int cid = snd_seq_client_info_get_client(cinfo);
        if (cid == snd_seq_client_id(c->seq)) continue;

        const char* cname = snd_seq_client_info_get_name(cinfo);
        snd_seq_port_info_set_client(pinfo, cid);
        snd_seq_port_info_set_port(pinfo, -1);
        while (snd_seq_query_next_port(c->seq, pinfo) >= 0) {
            const char* pname = snd_seq_port_info_get_name(pinfo);
            if (strstr(cname, nameSubstr) || strstr(pname, nameSubstr)) {
                int pid = snd_seq_port_info_get_port(pinfo);
                c->dev_client = cid;
                c->dev_port   = pid;
                // Subscribe to receive from device.
                snd_seq_connect_from(c->seq, c->local_port, cid, pid);
                // Subscribe to send to device.
                snd_seq_connect_to(c->seq, c->local_port, cid, pid);
                return 0;
            }
        }
    }
    return -1;
}

// alsa_list_ports writes the list of available client:port names into a
// caller-provided buffer.  Each name is at most 127 bytes.  Returns the
// number of entries written (up to max).
static int alsa_list_ports(alsaConn* c, char (*buf)[128], int max) {
    snd_seq_client_info_t* cinfo;
    snd_seq_port_info_t*   pinfo;
    snd_seq_client_info_alloca(&cinfo);
    snd_seq_port_info_alloca(&pinfo);

    int n = 0;
    snd_seq_client_info_set_client(cinfo, -1);
    while (snd_seq_query_next_client(c->seq, cinfo) >= 0 && n < max) {
        int cid = snd_seq_client_info_get_client(cinfo);
        if (cid == snd_seq_client_id(c->seq)) continue;
        const char* cname = snd_seq_client_info_get_name(cinfo);
        snd_seq_port_info_set_client(pinfo, cid);
        snd_seq_port_info_set_port(pinfo, -1);
        while (snd_seq_query_next_port(c->seq, pinfo) >= 0 && n < max) {
            int pid = snd_seq_port_info_get_port(pinfo);
            const char* pname = snd_seq_port_info_get_name(pinfo);
            snprintf(buf[n], 128, "%d:%d  %s / %s", cid, pid, cname, pname);
            n++;
        }
    }
    return n;
}

// alsa_send_sysex sends a SysEx message.  data must NOT include F0/F7.
static int alsa_send_sysex(alsaConn* c, const unsigned char* data, int len) {
    int total = len + 2;
    unsigned char* buf = malloc(total);
    if (!buf) return -ENOMEM;
    buf[0] = 0xF0;
    memcpy(buf + 1, data, len);
    buf[total - 1] = 0xF7;

    snd_seq_event_t ev;
    snd_seq_ev_clear(&ev);
    snd_seq_ev_set_source(&ev, c->local_port);
    snd_seq_ev_set_dest(&ev, c->dev_client, c->dev_port);
    snd_seq_ev_set_direct(&ev);
    snd_seq_ev_set_sysex(&ev, total, buf);
    int ret = snd_seq_event_output_direct(c->seq, &ev);
    free(buf);
    return (ret < 0) ? ret : 0;
}

// alsa_send_cc sends a MIDI CC message.
static int alsa_send_cc(alsaConn* c, int channel, int controller, int value) {
    snd_seq_event_t ev;
    snd_seq_ev_clear(&ev);
    snd_seq_ev_set_source(&ev, c->local_port);
    snd_seq_ev_set_dest(&ev, c->dev_client, c->dev_port);
    snd_seq_ev_set_direct(&ev);
    snd_seq_ev_set_controller(&ev, channel, controller, value);
    int ret = snd_seq_event_output_direct(c->seq, &ev);
    return (ret < 0) ? ret : 0;
}

// alsa_recv_sysex waits up to timeout_ms for a complete SysEx message.
// The ALSA sequencer may fragment large SysEx messages across multiple
// SND_SEQ_EVENT_SYSEX events; this function accumulates all chunks until
// the F7 terminator is seen.
// On success, allocates *out (WITHOUT F0/F7), sets *out_len, returns 0.
// Returns -1 on timeout.  Caller must free(*out).
static int alsa_recv_sysex(alsaConn* c, int timeout_ms,
                            unsigned char** out, int* out_len) {
    unsigned char* accum = NULL;
    int            accum_len = 0;
    int            complete = 0;

    int npfd = snd_seq_poll_descriptors_count(c->seq, POLLIN);
    struct pollfd* pfds = malloc(npfd * sizeof(struct pollfd));
    if (!pfds) return -ENOMEM;
    snd_seq_poll_descriptors(c->seq, pfds, npfd, POLLIN);

    // Compute absolute deadline.
    struct timespec deadline;
    clock_gettime(CLOCK_MONOTONIC, &deadline);
    deadline.tv_sec  += timeout_ms / 1000;
    deadline.tv_nsec += (long)(timeout_ms % 1000) * 1000000L;
    if (deadline.tv_nsec >= 1000000000L) {
        deadline.tv_sec++;
        deadline.tv_nsec -= 1000000000L;
    }

    while (!complete) {
        struct timespec now;
        clock_gettime(CLOCK_MONOTONIC, &now);
        long rem_ms = (long)(deadline.tv_sec - now.tv_sec) * 1000L
                    + (deadline.tv_nsec - now.tv_nsec) / 1000000L;
        if (rem_ms <= 0) break;

        int r = poll(pfds, npfd, (int)rem_ms);
        if (r <= 0) break; // timeout or error

        // Drain all pending events.
        snd_seq_event_t* ev;
        while (!complete && snd_seq_event_input(c->seq, &ev) > 0) {
            if (ev->type != SND_SEQ_EVENT_SYSEX) continue;

            unsigned char* data = (unsigned char*)ev->data.ext.ptr;
            int len = (int)ev->data.ext.len;

            // First chunk begins with F0 — strip it.
            int start = (accum_len == 0 && len > 0 && data[0] == 0xF0) ? 1 : 0;
            // Last chunk ends with F7 — strip it and mark done.
            int has_f7 = (len > 0 && data[len - 1] == 0xF7) ? 1 : 0;
            int end    = has_f7 ? len - 1 : len;
            int chunk  = end - start;

            if (chunk > 0) {
                unsigned char* tmp = realloc(accum, accum_len + chunk);
                if (!tmp) {
                    free(accum);
                    free(pfds);
                    return -ENOMEM;
                }
                accum = tmp;
                memcpy(accum + accum_len, data + start, chunk);
                accum_len += chunk;
            }
            if (has_f7) complete = 1;
        }
    }

    free(pfds);
    if (complete) {
        *out     = accum;
        *out_len = accum_len;
        return 0;
    }
    free(accum);
    return -1; // timeout
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"nanoctl/internal/scene"
)

// SysEx payloads — without F0/F7.
var (
	sysexQuery = []byte{0x42, 0x40, 0x00, 0x01, 0x13, 0x00, 0x1F, 0x10, 0x00}
	sysexSave  = []byte{0x42, 0x40, 0x00, 0x01, 0x13, 0x00, 0x1F, 0x11, 0x00}

	sysexWritePrefix = []byte{0x42, 0x40, 0x00, 0x01, 0x13, 0x00, 0x7F, 0x7F, 0x02, 0x03, 0x05, 0x40}

	respHeader        = []byte{0x00, 0x01, 0x13, 0x00}
	funcSceneDump     = []byte{0x7F, 0x7F, 0x02, 0x03, 0x05, 0x40}
	funcACK           = []byte{0x5F, 0x23, 0x00}
	funcWriteComplete = []byte{0x5F, 0x21, 0x00}
	funcWriteError    = []byte{0x5F, 0x22, 0x00}
)

// MidiConn wraps an ALSA sequencer connection to the nanoKONTROL2.
type MidiConn struct {
	c *C.alsaConn
}

// OpenPorts opens the ALSA sequencer and connects to the port matching portName.
func OpenPorts(portName string) (conn *MidiConn, cleanup func(), err error) {
	cname := C.CString("nanoctl")
	defer C.free(unsafe.Pointer(cname))

	ac := C.alsa_open(cname)
	if ac == nil {
		return nil, nil, fmt.Errorf("failed to open ALSA sequencer")
	}

	csub := C.CString(portName)
	defer C.free(unsafe.Pointer(csub))

	if ret := C.alsa_find_port(ac, csub); ret != 0 {
		C.alsa_close(ac)
		return nil, nil, fmt.Errorf("no MIDI port matching %q — use --list-ports to see available ports", portName)
	}

	conn = &MidiConn{c: ac}
	cleanup = func() { C.alsa_close(ac) }
	return conn, cleanup, nil
}

// sendSysex sends a SysEx payload (without F0/F7) to the device.
func (m *MidiConn) sendSysex(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	ret := C.alsa_send_sysex(m.c, (*C.uchar)(unsafe.Pointer(&data[0])), C.int(len(data)))
	if ret < 0 {
		return fmt.Errorf("ALSA send SysEx error: %d", int(ret))
	}
	return nil
}

// sendCC sends a MIDI CC message.
func (m *MidiConn) sendCC(channel, controller, value int) error {
	ret := C.alsa_send_cc(m.c, C.int(channel), C.int(controller), C.int(value))
	if ret < 0 {
		return fmt.Errorf("ALSA send CC error: %d", int(ret))
	}
	return nil
}

// SendCC sends a MIDI CC message via conn.
func SendCC(conn *MidiConn, ch, cc, val int) error {
	return conn.sendCC(ch, cc, val)
}

// recvSysex waits up to the given duration for a SysEx event.
// Returns the payload without F0/F7.
func (m *MidiConn) recvSysex(timeout time.Duration) ([]byte, error) {
	ms := C.int(timeout.Milliseconds())
	var cdata *C.uchar
	var clen C.int
	ret := C.alsa_recv_sysex(m.c, ms, &cdata, &clen)
	if ret != 0 {
		return nil, fmt.Errorf("timeout waiting for SysEx response (is the device connected and powered on?)")
	}
	defer C.free(unsafe.Pointer(cdata))
	result := make([]byte, int(clen))
	copy(result, unsafe.Slice((*byte)(unsafe.Pointer(cdata)), int(clen)))
	return result, nil
}

// waitForSysEx sends a message via ready(), then waits for a matching SysEx response.
func (m *MidiConn) waitForSysEx(ready func() error, match func([]byte) bool, timeout time.Duration) ([]byte, error) {
	if err := ready(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timeout waiting for SysEx response")
		}
		data, err := m.recvSysex(remaining)
		if err != nil {
			return nil, err
		}
		if match(data) {
			return data, nil
		}
	}
}

// isKorgResponse returns true if data (without F0/F7) is a nanoKONTROL2 response.
func isKorgResponse(data []byte) bool {
	return len(data) >= 6 &&
		data[0] == 0x42 &&
		(data[1]&0xF0) == 0x40 &&
		bytes.Equal(data[2:6], respHeader)
}

func responseFunc(data []byte) []byte {
	if len(data) < 7 {
		return nil
	}
	return data[6:]
}

// QueryScene sends a scene-dump request and returns the 339-byte decoded scene data
// together with the parsed Scene struct.
func QueryScene(m *MidiConn) ([]byte, *scene.Scene, error) {
	isSceneDump := func(data []byte) bool {
		if !isKorgResponse(data) {
			return false
		}
		fn := responseFunc(data)
		return len(fn) >= len(funcSceneDump) && bytes.Equal(fn[:len(funcSceneDump)], funcSceneDump)
	}

	resp, err := m.waitForSysEx(func() error {
		return m.sendSysex(sysexQuery)
	}, isSceneDump, 3*time.Second)
	if err != nil {
		return nil, nil, err
	}

	// Payload (without F0/F7):
	//   [0]     0x42
	//   [1]     0x4g
	//   [2..5]  00 01 13 00
	//   [6..11] 7F 7F 02 03 05 40
	//   [12..399] 388 encoded bytes
	headerLen := 2 + len(respHeader) + len(funcSceneDump) // = 12
	if len(resp) < headerLen+388 {
		return nil, nil, fmt.Errorf("scene dump too short: %d bytes (expected %d)", len(resp), headerLen+388)
	}

	encoded := resp[headerLen : headerLen+388]
	decoded := scene.Decode7bit(encoded)

	s := scene.DecodeScene(decoded)
	if s == nil {
		return nil, nil, fmt.Errorf("decode scene: buffer too short")
	}
	return decoded, s, nil
}

// WriteScene sends scene data to the device and waits for save confirmation.
func WriteScene(m *MidiConn, decoded []byte) error {
	encoded := scene.Encode7bit(decoded)
	writePayload := append(append([]byte(nil), sysexWritePrefix...), encoded...)

	isACK := func(data []byte) bool {
		if !isKorgResponse(data) {
			return false
		}
		fn := responseFunc(data)
		return len(fn) >= len(funcACK) && bytes.Equal(fn[:len(funcACK)], funcACK)
	}
	isWriteDone := func(data []byte) bool {
		if !isKorgResponse(data) {
			return false
		}
		fn := responseFunc(data)
		return (len(fn) >= len(funcWriteComplete) && bytes.Equal(fn[:len(funcWriteComplete)], funcWriteComplete)) ||
			(len(fn) >= len(funcWriteError) && bytes.Equal(fn[:len(funcWriteError)], funcWriteError))
	}

	if _, err := m.waitForSysEx(func() error {
		return m.sendSysex(writePayload)
	}, isACK, 3*time.Second); err != nil {
		return fmt.Errorf("waiting for ACK: %w", err)
	}

	resp, err := m.waitForSysEx(func() error {
		return m.sendSysex(sysexSave)
	}, isWriteDone, 3*time.Second)
	if err != nil {
		return fmt.Errorf("waiting for write-complete: %w", err)
	}

	fn := responseFunc(resp)
	if len(fn) >= len(funcWriteError) && bytes.Equal(fn[:len(funcWriteError)], funcWriteError) {
		return fmt.Errorf("device reported write error")
	}
	return nil
}

// ListPorts prints all available ALSA MIDI sequencer ports.
func ListPorts(portName string) {
	cname := C.CString("nanoctl-list")
	defer C.free(unsafe.Pointer(cname))
	ac := C.alsa_open(cname)
	if ac == nil {
		fmt.Println("Failed to open ALSA sequencer.")
		return
	}
	defer C.alsa_close(ac)

	const maxPorts = 128
	var buf [maxPorts][128]C.char
	n := int(C.alsa_list_ports(ac, &buf[0], C.int(maxPorts)))

	fmt.Printf("MIDI ports (searching for %q):\n", portName)
	for i := range n {
		name := C.GoString(&buf[i][0])
		marker := ""
		if strings.Contains(name, portName) {
			marker = "  ← match"
		}
		fmt.Printf("  %s%s\n", name, marker)
	}
}
