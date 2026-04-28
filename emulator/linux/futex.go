//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

const (
	FutexWait          = 0
	FutexWake          = 1
	FutexFd            = 2
	FutexRequeue       = 3
	FutexCmpRequeue    = 4
	FutexWakeOp        = 5
	FutexLockPi        = 6
	FutexUnlockPi      = 7
	FutexTrylockPi     = 8
	FutexWaitBitset    = 9
	FutexWakeBitset    = 10
	FutexWaitRequeuePi = 11
	FutexCmpRequeuePi  = 12
	FutexLockPi2       = 13

	FutexPrivateFlag   = 128
	FutexClockRealtime = 256
	FutexCmdMask       = ^(FutexPrivateFlag | FutexClockRealtime)

	FutexWaitPrivate          = FutexWait | FutexPrivateFlag
	FutexWakePrivate          = FutexWake | FutexPrivateFlag
	FutexRequeuePrivate       = FutexRequeue | FutexPrivateFlag
	FutexCmpRequeuePrivate    = FutexCmpRequeue | FutexPrivateFlag
	FutexWakeOpPrivate        = FutexWakeOp | FutexPrivateFlag
	FutexLockPiPrivate        = FutexLockPi | FutexPrivateFlag
	FutexLockPi2Private       = FutexLockPi2 | FutexPrivateFlag
	FutexUnlockPiPrivate      = FutexUnlockPi | FutexPrivateFlag
	FutexTrylockPiPrivate     = FutexTrylockPi | FutexPrivateFlag
	FutexWaitBitsetPrivate    = FutexWaitBitset | FutexPrivateFlag
	FutexWakeBitsetPrivate    = FutexWakeBitset | FutexPrivateFlag
	FutexWaitRequeuePiPrivate = FutexWaitRequeuePi | FutexPrivateFlag
	FutexCmpRequeuePiPrivate  = FutexCmpRequeuePi | FutexPrivateFlag
)
