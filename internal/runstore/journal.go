package runstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"
)

const maxJournalRecordBytes = 4 << 20

var errJournalRecordTooLarge = errors.New("journal record exceeds size limit")

// JournalRecordLimitError reports a rejected pre-commit journal record without
// collapsing it into a generic storage mutation failure.
type JournalRecordLimitError struct {
	Context string
	Size    int
	Limit   int
}

func (err *JournalRecordLimitError) Error() string {
	if err == nil {
		return "journal record exceeds size limit"
	}
	return fmt.Sprintf("%s exceeds journal record limit: %d bytes exceeds %d", err.Context, err.Size, err.Limit)
}

// JournalRecordContext returns the operation whose encoded record exceeded the limit.
func (err *JournalRecordLimitError) JournalRecordContext() string { return err.Context }

// JournalRecordSize returns the rejected encoded byte size.
func (err *JournalRecordLimitError) JournalRecordSize() int { return err.Size }

// JournalRecordLimit returns the maximum accepted encoded byte size.
func (err *JournalRecordLimitError) JournalRecordLimit() int { return err.Limit }

type Event struct {
	Sequence int             `json:"sequence"`
	Type     string          `json:"type"`
	At       time.Time       `json:"at"`
	Data     json.RawMessage `json:"data"`
}

func NewEvent(eventType string, value any) (Event, error) {
	if eventType == "" {
		return Event{}, errors.New("event type is required")
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return Event{}, fmt.Errorf("encode event data: %w", err)
	}
	data := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	if limit := maxJournalRecordBytes - (64 << 10); len(data) > limit {
		return Event{}, &JournalRecordLimitError{Context: "event payload", Size: len(data), Limit: limit}
	}
	return Event{Type: eventType, At: time.Now().UTC(), Data: append(json.RawMessage(nil), data...)}, nil
}

type journalContext struct {
	root     *os.Root
	hooks    storeHooks
	tracker  *mutationTracker
	validate func(MutationPhase, faultPoint) error
}

func (transaction *runTransaction) journalContext() journalContext {
	return journalContext{
		root:     transaction.run.root,
		hooks:    transaction.run.store.hooks,
		tracker:  transaction.tracker,
		validate: transaction.validate,
	}
}

func (run *runHandle) readOnlyJournalContext() journalContext {
	return journalContext{
		root:  run.root,
		hooks: run.store.hooks,
		validate: func(_ MutationPhase, point faultPoint) error {
			if point != faultValidateRun {
				if err := run.store.hooks.at(point); err != nil {
					return err
				}
			}
			return run.validate()
		},
	}
}

func (context journalContext) check(phase MutationPhase, point faultPoint) error {
	if context.validate == nil {
		return errors.New("journal namespace validator is required")
	}
	return context.validate(phase, point)
}

// visitJournal decodes one JSONL record at a time so recovery memory is bounded
// by the largest event rather than by the complete run history. Tail repair uses
// the same verified read/write handle that performed the scan.
func visitJournal(context journalContext, name string, visit func(Event) error) (int, error) {
	return visitJournalMode(context, name, visit, true)
}

func visitJournalReadOnly(context journalContext, name string, visit func(Event) error) (int, error) {
	return visitJournalMode(context, name, visit, false)
}

func visitJournalMode(context journalContext, name string, visit func(Event) error, repair bool) (int, error) {
	if err := context.check(PhaseReplay, faultValidateRun); err != nil {
		return 0, err
	}
	flags := os.O_RDONLY
	if repair {
		flags = os.O_RDWR
	}
	file, _, err := openRegularFileInRoot(context.root, name, flags, 0o600, false)
	if errors.Is(err, fs.ErrNotExist) {
		if validateErr := context.check(PhaseReplay, faultValidateRun); validateErr != nil {
			return 0, validateErr
		}
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("open journal: %w", err)
	}
	defer file.Close()
	if err := context.hooks.at(faultJournalOpened); err != nil {
		return 0, fmt.Errorf("inspect opened journal: %w", err)
	}
	if err := verifyJournalHandle(context, name, file, PhaseReplay); err != nil {
		return 0, err
	}

	reader := bufio.NewReaderSize(file, 64<<10)
	count := 0
	var offset int64
	var truncateAt int64 = -1
	appendNewline := false
	for {
		recordOffset := offset
		line, consumed, complete, readErr := readBoundedJournalRecord(reader)
		offset += consumed
		if errors.Is(readErr, errJournalRecordTooLarge) {
			if !complete {
				truncateAt = recordOffset
				break
			}
			return 0, journalReplayFailure(context, name, file, fmt.Errorf("decode journal event %d: record exceeds %d bytes", count+1, maxJournalRecordBytes))
		}
		if len(bytes.TrimSpace(line)) > 0 {
			event, decodeErr := decodeJournalEvent(line, count+1)
			if decodeErr != nil {
				if errors.Is(readErr, io.EOF) && !complete {
					truncateAt = recordOffset
					break
				}
				return 0, journalReplayFailure(context, name, file, decodeErr)
			}
			if err := visit(event); err != nil {
				return 0, journalReplayFailure(context, name, file, err)
			}
			count++
			if err := verifyJournalHandle(context, name, file, PhaseReplay); err != nil {
				return 0, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			if consumed > 0 && !complete {
				appendNewline = true
			}
			break
		}
		if readErr != nil {
			return 0, journalReplayFailure(context, name, file, fmt.Errorf("read journal: %w", readErr))
		}
	}
	if repair && (truncateAt >= 0 || appendNewline) {
		if truncateAt >= 0 {
			appendNewline = false
		}
		if err := repairJournalTail(context, name, file, truncateAt, appendNewline); err != nil {
			return 0, err
		}
	}
	if err := verifyJournalHandle(context, name, file, PhaseReplay); err != nil {
		return 0, err
	}
	return count, nil
}

func readFirstJournalEvent(context journalContext, name string) (Event, error) {
	if err := context.check(PhaseReplay, faultValidateRun); err != nil {
		return Event{}, err
	}
	file, _, err := openRegularFileInRoot(context.root, name, os.O_RDONLY, 0o600, false)
	if errors.Is(err, fs.ErrNotExist) {
		return Event{}, errors.New("run journal is missing")
	}
	if err != nil {
		return Event{}, fmt.Errorf("open journal: %w", err)
	}
	defer file.Close()
	if err := context.hooks.at(faultJournalOpened); err != nil {
		return Event{}, fmt.Errorf("inspect opened journal: %w", err)
	}
	if err := verifyJournalHandle(context, name, file, PhaseReplay); err != nil {
		return Event{}, err
	}
	line, _, complete, readErr := readBoundedJournalRecord(bufio.NewReaderSize(file, 64<<10))
	if errors.Is(readErr, errJournalRecordTooLarge) {
		return Event{}, fmt.Errorf("decode journal event 1: record exceeds %d bytes", maxJournalRecordBytes)
	}
	if errors.Is(readErr, io.EOF) && len(line) == 0 {
		return Event{}, errors.New("run journal is empty")
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return Event{}, fmt.Errorf("read first journal event: %w", readErr)
	}
	event, err := decodeJournalEvent(line, 1)
	if err != nil {
		if !complete {
			return Event{}, fmt.Errorf("first journal event is incomplete: %w", err)
		}
		return Event{}, err
	}
	if err := verifyJournalHandle(context, name, file, PhaseReplay); err != nil {
		return Event{}, err
	}
	return event, nil
}

func readBoundedJournalRecord(reader *bufio.Reader) (line []byte, consumed int64, complete bool, err error) {
	var buffer bytes.Buffer
	tooLarge := false
	for {
		fragment, readErr := reader.ReadSlice('\n')
		consumed += int64(len(fragment))
		if !tooLarge {
			if buffer.Len()+len(fragment) > maxJournalRecordBytes {
				tooLarge = true
				buffer.Reset()
			} else {
				_, _ = buffer.Write(fragment)
			}
		}
		switch {
		case readErr == nil:
			if tooLarge {
				return nil, consumed, true, errJournalRecordTooLarge
			}
			return buffer.Bytes(), consumed, true, nil
		case errors.Is(readErr, bufio.ErrBufferFull):
			continue
		case errors.Is(readErr, io.EOF):
			if consumed == 0 {
				return nil, 0, false, io.EOF
			}
			if tooLarge {
				return nil, consumed, false, errJournalRecordTooLarge
			}
			return buffer.Bytes(), consumed, false, io.EOF
		default:
			return nil, consumed, false, readErr
		}
	}
}

func decodeJournalEvent(line []byte, expectedSequence int) (Event, error) {
	if err := validateJournalJSONStructure(line); err != nil {
		return Event{}, fmt.Errorf("decode journal event %d: %w", expectedSequence, err)
	}
	var event Event
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return Event{}, fmt.Errorf("decode journal event %d: %w", expectedSequence, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Event{}, fmt.Errorf("decode journal event %d: multiple JSON values", expectedSequence)
		}
		return Event{}, fmt.Errorf("decode journal event %d: %w", expectedSequence, err)
	}
	if event.Sequence != expectedSequence || event.Type == "" || event.At.IsZero() || len(event.Data) == 0 {
		return Event{}, fmt.Errorf("invalid journal event %d", expectedSequence)
	}
	return event, nil
}

func repairJournalTail(context journalContext, name string, file *os.File, truncateAt int64, appendNewline bool) error {
	if err := context.hooks.at(faultTailScanBeforeRepair); err != nil {
		return fmt.Errorf("repair journal tail: %w", err)
	}
	if err := verifyJournalHandle(context, name, file, PhaseReplay); err != nil {
		return err
	}
	if truncateAt >= 0 {
		if err := file.Truncate(truncateAt); err != nil {
			return fmt.Errorf("truncate interrupted journal record: %w", err)
		}
	}
	if appendNewline {
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			return fmt.Errorf("seek journal for repair: %w", err)
		}
		if _, err := file.Write([]byte{'\n'}); err != nil {
			return fmt.Errorf("terminate journal record: %w", err)
		}
	}
	if err := context.hooks.at(faultTailAfterRepair); err != nil {
		return fmt.Errorf("repair journal tail: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync repaired journal: %w", err)
	}
	return verifyJournalHandle(context, name, file, PhaseReplay)
}

func encodeJournalEvents(events []Event) ([]byte, error) {
	if len(events) == 0 {
		return nil, errors.New("append journal: at least one event is required")
	}
	var encoded bytes.Buffer
	for _, event := range events {
		var record bytes.Buffer
		encoder := json.NewEncoder(&record)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(event); err != nil {
			return nil, fmt.Errorf("encode journal event: %w", err)
		}
		if record.Len() > maxJournalRecordBytes {
			return nil, &JournalRecordLimitError{Context: "encoded journal event", Size: record.Len(), Limit: maxJournalRecordBytes}
		}
		_, _ = encoded.Write(record.Bytes())
	}
	return encoded.Bytes(), nil
}

func appendJournal(context journalContext, name string, events []Event) error {
	encoded, err := encodeJournalEvents(events)
	if err != nil {
		return err
	}
	return appendEncodedJournalContext(context, name, encoded)
}

func appendEncodedJournal(transaction *runTransaction, name string, encoded []byte) error {
	return appendEncodedJournalContext(transaction.journalContext(), name, encoded)
}

func appendEncodedJournalContext(context journalContext, name string, encoded []byte) error {
	if len(encoded) == 0 {
		return context.tracker.fail(PhaseJournalWrite, false, errors.New("append journal: encoded records are empty"))
	}
	if err := context.check(PhaseJournalOpen, faultValidateRun); err != nil {
		return err
	}
	journalIdentity, err := inspectRegularFileOrMissingInRoot(context.root, name)
	if err != nil {
		return context.tracker.fail(PhaseJournalOpen, false, fmt.Errorf("inspect journal: %w", err))
	}
	if !journalIdentity.exists {
		if err := context.hooks.at(faultCreateJournal); err != nil {
			return context.tracker.fail(PhaseJournalOpen, false, err)
		}
	}
	file, _, err := openRegularFileInRoot(context.root, name, os.O_WRONLY|os.O_APPEND, 0o600, true)
	if err != nil {
		return context.tracker.fail(PhaseJournalOpen, false, fmt.Errorf("open journal: %w", err))
	}
	defer file.Close()
	if err := restrictPrivateFile(file, 0o600); err != nil {
		return context.tracker.fail(PhaseJournalOpen, false, fmt.Errorf("secure journal ACL: %w", err))
	}
	if err := context.hooks.at(faultJournalOpened); err != nil {
		return context.tracker.fail(PhaseJournalVerify, false, err)
	}
	if err := file.Chmod(0o600); err != nil {
		return context.tracker.fail(PhaseJournalOpen, false, fmt.Errorf("secure journal: %w", err))
	}
	if err := verifyJournalHandle(context, name, file, PhaseJournalVerify); err != nil {
		return err
	}
	n, writeErr := file.Write(encoded)
	if n > 0 {
		context.tracker.markJournalWrite()
	}
	if writeErr != nil {
		return context.tracker.fail(PhaseJournalWrite, false, fmt.Errorf("append journal: %w", writeErr))
	}
	if n != len(encoded) {
		return context.tracker.fail(PhaseJournalWrite, false, io.ErrShortWrite)
	}
	if err := context.hooks.at(faultJournalAfterWrite); err != nil {
		return context.tracker.fail(PhaseJournalWrite, false, err)
	}
	if err := verifyJournalHandle(context, name, file, PhaseJournalVerify); err != nil {
		return context.tracker.fail(PhaseJournalVerify, true, err)
	}
	if err := context.hooks.at(faultJournalBeforeSync); err != nil {
		return context.tracker.fail(PhaseJournalSync, false, err)
	}
	if err := file.Sync(); err != nil {
		return context.tracker.fail(PhaseJournalSync, false, fmt.Errorf("sync journal: %w", err))
	}
	if journalIdentity.exists {
		context.tracker.markJournalCommitted()
	}
	if err := context.hooks.at(faultJournalAfterSync); err != nil {
		return context.tracker.fail(PhaseJournalVerify, false, err)
	}
	if err := verifyJournalHandle(context, name, file, PhaseJournalVerify); err != nil {
		return context.tracker.fail(PhaseJournalVerify, true, err)
	}
	return nil
}

func verifyJournalHandle(context journalContext, name string, file *os.File, phase MutationPhase) error {
	if err := verifyOpenedRegularFileInRoot(context.root, name, file); err != nil {
		return context.tracker.fail(phase, context.tracker != nil && context.tracker.wroteJournal, fmt.Errorf("verify journal leaf: %w", err))
	}
	if err := context.check(phase, faultValidateRun); err != nil {
		return err
	}
	return nil
}

func journalReplayFailure(context journalContext, name string, file *os.File, cause error) error {
	if verifyErr := verifyJournalHandle(context, name, file, PhaseReplay); verifyErr != nil {
		return errors.Join(cause, verifyErr)
	}
	return cause
}
