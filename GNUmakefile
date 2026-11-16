all: GoEMU.app/Contents/MacOS/goemu GoEMU.app/Contents/Resources/AppIcon.icns

GoEMU.app/Contents/MacOS/goemu: cmd/goemu/goemu
	cp $+ $@

GoEMU.app/Contents/Resources/AppIcon.icns: docs/goemu.png
	-mkdir MyIcon.iconset
	sips -z 16 16     $+ --out MyIcon.iconset/icon_16x16.png
	sips -z 32 32     $+ --out MyIcon.iconset/icon_16x16@2x.png
	sips -z 32 32     $+ --out MyIcon.iconset/icon_32x32.png
	sips -z 64 64     $+ --out MyIcon.iconset/icon_32x32@2x.png
	sips -z 128 128   $+ --out MyIcon.iconset/icon_128x128.png
	sips -z 256 256   $+ --out MyIcon.iconset/icon_128x128@2x.png
	sips -z 256 256   $+ --out MyIcon.iconset/icon_256x256.png
	sips -z 512 512   $+ --out MyIcon.iconset/icon_256x256@2x.png
	sips -z 512 512   $+ --out MyIcon.iconset/icon_512x512.png
	sips -z 1024 1024 $+ --out MyIcon.iconset/icon_512x512@2x.png
	iconutil -c icns MyIcon.iconset -o $@
	-rm -rf ./MyIcon.iconset

wc:
	@./wc.sh
