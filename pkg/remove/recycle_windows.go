//go:build windows

package remove

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32     = windows.NewLazySystemDLL("ole32.dll")
	shell32   = windows.NewLazySystemDLL("shell32.dll")
	comInit   = ole32.NewProc("CoInitializeEx")
	comUninit = ole32.NewProc("CoUninitialize")
	comCreate = ole32.NewProc("CoCreateInstance")
	shellItem = shell32.NewProc("SHCreateItemFromParsingName")

	clsidFileOperation = windows.GUID{Data1: 0x3ad05575, Data2: 0x8857, Data3: 0x4850, Data4: [8]byte{0x92, 0x77, 0x11, 0xb8, 0x5b, 0xdb, 0x8e, 0x09}}
	iidFileOperation   = windows.GUID{Data1: 0x947aab5f, Data2: 0x0a5c, Data3: 0x4c13, Data4: [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	iidShellItem       = windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
	iidProgressSink    = windows.GUID{Data1: 0x04b0f1a7, Data2: 0x9490, Data3: 0x44bc, Data4: [8]byte{0x96, 0xe1, 0x42, 0x96, 0xa3, 0x12, 0x52, 0xe2}}
	iidUnknown         = windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

const (
	fofSilent           = 0x0004
	fofNoErrorUI        = 0x0400
	fofNoConnectedItems = 0x2000
	fofWantNukeWarning  = 0x4000
	fofxRecycleOnDelete = 0x00080000
	fofxEarlyFailure    = 0x00100000
	tsfRecycle          = 0x80
	eAbort              = 0x80004004
	eNoInterface        = 0x80004002
)

type shellObject struct {
	vtable *[23]uintptr
}

func (object *shellObject) call(method int, args ...uintptr) uintptr {
	all := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	result, _, _ := syscall.SyscallN(object.vtable[method], all...)
	return result
}

type recycleSink struct {
	vtable  *[19]uintptr
	refs    uintptr
	refused bool
	posted  bool
	result  uintptr
	inBin   bool
}

func recycleQuery(sink *recycleSink, iid *windows.GUID, out **recycleSink) uintptr {
	*out = nil
	if *iid != iidUnknown && *iid != iidProgressSink {
		return eNoInterface
	}
	*out = sink
	recycleAddRef(sink)
	return 0
}

func recycleAddRef(sink *recycleSink) uintptr {
	sink.refs++
	return sink.refs
}

func recycleRelease(sink *recycleSink) uintptr {
	sink.refs--
	return sink.refs
}

func recyclePreDelete(sink *recycleSink, flags, _ uintptr) uintptr {
	// Abort before the Shell acts if the operation has switched to a
	// permanent delete. FOFX_RECYCLEONDELETE also disallows that fallback.
	if flags&tsfRecycle == 0 {
		sink.refused = true
		return eAbort
	}
	return 0
}

func recyclePostDelete(sink *recycleSink, _, _, result, destination uintptr) uintptr {
	sink.posted = true
	sink.result = result
	sink.inBin = destination != 0
	return 0
}

var recycleCallbacks = func() [19]uintptr {
	one := syscall.NewCallback(func(_ uintptr) uintptr { return 0 })
	two := syscall.NewCallback(func(_, _ uintptr) uintptr { return 0 })
	three := syscall.NewCallback(func(_, _, _ uintptr) uintptr { return 0 })
	four := syscall.NewCallback(func(_, _, _, _ uintptr) uintptr { return 0 })
	five := syscall.NewCallback(func(_, _, _, _, _ uintptr) uintptr { return 0 })
	six := syscall.NewCallback(func(_, _, _, _, _, _ uintptr) uintptr { return 0 })
	seven := syscall.NewCallback(func(_, _, _, _, _, _, _ uintptr) uintptr { return 0 })
	eight := syscall.NewCallback(func(_, _, _, _, _, _, _, _ uintptr) uintptr { return 0 })
	return [19]uintptr{
		syscall.NewCallback(recycleQuery), syscall.NewCallback(recycleAddRef), syscall.NewCallback(recycleRelease),
		one, two, four, six, five, seven, five, seven,
		syscall.NewCallback(recyclePreDelete), syscall.NewCallback(recyclePostDelete),
		four, eight, three, one, one, one,
	}
}()

func shellError(stage string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s（Windows 错误 0x%08X）", stage, uint32(hr))
	}
	return nil
}

// RecycleWindowsFile uses the Windows Shell and requires confirmation from its
// per-file callback that a Recycle Bin item was created. There is no fallback
// to an ordinary delete.
func RecycleWindowsFile(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := comInit.Call(0, 2) // COINIT_APARTMENTTHREADED
	if err := shellError("无法连接 Windows 回收站", hr); err != nil {
		return err
	}
	defer comUninit.Call()

	var operation, item *shellObject
	hr, _, _ = comCreate.Call(uintptr(unsafe.Pointer(&clsidFileOperation)), 0, 1,
		uintptr(unsafe.Pointer(&iidFileOperation)), uintptr(unsafe.Pointer(&operation)))
	if err := shellError("无法启动 Windows 文件操作", hr); err != nil {
		return err
	}
	defer operation.call(2) // IUnknown::Release

	hr, _, _ = shellItem.Call(uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if err := shellError("无法打开待清理文件", hr); err != nil {
		return err
	}
	defer item.call(2)

	flags := uintptr(fofSilent | fofNoErrorUI | fofNoConnectedItems | fofWantNukeWarning | fofxRecycleOnDelete | fofxEarlyFailure)
	if err := shellError("无法设置回收站模式", operation.call(5, flags)); err != nil {
		return err
	}

	sink := &recycleSink{vtable: &recycleCallbacks, refs: 1}
	var pin runtime.Pinner
	pin.Pin(sink)
	defer pin.Unpin()
	var cookie uint32
	if err := shellError("无法监听回收结果", operation.call(3, uintptr(unsafe.Pointer(sink)), uintptr(unsafe.Pointer(&cookie)))); err != nil {
		return err
	}

	defer operation.call(4, uintptr(cookie)) // Unadvise before unpinning the sink.

	if err := shellError("无法加入回收队列", operation.call(18, uintptr(unsafe.Pointer(item)), 0)); err != nil {
		return err
	}
	performed := operation.call(21)
	if sink.refused {
		return fmt.Errorf("Windows 无法将此文件移入回收站")
	}
	if err := shellError("回收站操作失败", performed); err != nil {
		return err
	}
	if err := shellError("文件未能移入回收站", sink.result); err != nil {
		return err
	}
	if !sink.posted || !sink.inBin {
		return fmt.Errorf("Windows 未确认文件已移入回收站")
	}
	return nil
}
