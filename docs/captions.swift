// Draws the demo video's captions larger than a terminal can. It reads the
// times the demo logged (DEMO_CAPTIONS: seconds, a tab, the caption; the
// last line has no caption and marks the end), draws each caption as a
// strip the width of the video, and writes the ffmpeg filter that pads the
// video with a strip below it and lays each caption there in its turn.
// VHS drops the odd frame, so its recording runs a little shorter than the
// demo did: the times are scaled to the video's length to stay in step.
//
// swift docs/captions.swift <times.tsv> <out dir> <video width> <video seconds> <font file>

import CoreGraphics
import CoreText
import Foundation
import ImageIO
import UniformTypeIdentifiers

let args = CommandLine.arguments
guard args.count == 6, let width = Int(args[3]), let seconds = Double(args[4]) else {
    FileHandle.standardError.write("usage: captions.swift <times.tsv> <out dir> <video width> <video seconds> <font file>\n".data(using: .utf8)!)
    exit(2)
}
let (timesPath, outDir, fontPath) = (args[1], args[2], args[5])

let stripH = 112
let fontSize: CGFloat = 36

// Catppuccin Macchiato, as brewery's default theme has it.
func rgb(_ hex: UInt32) -> CGColor {
    CGColor(srgbRed: CGFloat(hex >> 16 & 0xff) / 255, green: CGFloat(hex >> 8 & 0xff) / 255,
            blue: CGFloat(hex & 0xff) / 255, alpha: 1)
}
let mantle: UInt32 = 0x1e2030
let (text, mauve) = (rgb(0xcad3f5), rgb(0xc6a0f6))

guard let descs = CTFontManagerCreateFontDescriptorsFromURL(URL(fileURLWithPath: fontPath) as CFURL) as? [CTFontDescriptor],
      let desc = descs.first else {
    FileHandle.standardError.write("can't read the font \(fontPath)\n".data(using: .utf8)!)
    exit(1)
}
let font = CTFontCreateWithFontDescriptor(desc, fontSize, nil)

// draw writes caption as a PNG strip at path.
func draw(_ caption: String, to path: String) {
    let ctx = CGContext(data: nil, width: width, height: stripH, bitsPerComponent: 8, bytesPerRow: 0,
                        space: CGColorSpace(name: CGColorSpace.sRGB)!,
                        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    ctx.setFillColor(rgb(mantle))
    ctx.fill(CGRect(x: 0, y: 0, width: width, height: stripH))

    let line = CTLineCreateWithAttributedString(NSAttributedString(string: caption, attributes: [
        NSAttributedString.Key(kCTFontAttributeName as String): font,
        NSAttributedString.Key(kCTForegroundColorAttributeName as String): text,
    ]))
    var ascent: CGFloat = 0, descent: CGFloat = 0
    let textW = CGFloat(CTLineGetTypographicBounds(line, &ascent, &descent, nil))
    let barW: CGFloat = 6, gap: CGFloat = 20
    let x = (CGFloat(width) - (barW + gap + textW)) / 2
    let baseline = (CGFloat(stripH) - ascent - descent) / 2 + descent

    ctx.setFillColor(mauve)
    ctx.fill(CGRect(x: x, y: baseline - descent, width: barW, height: ascent + descent))
    ctx.textPosition = CGPoint(x: x + barW + gap, y: baseline)
    CTLineDraw(line, ctx)

    let dest = CGImageDestinationCreateWithURL(URL(fileURLWithPath: path) as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(dest, ctx.makeImage()!, nil)
    CGImageDestinationFinalize(dest)
}

let lines = try String(contentsOfFile: timesPath, encoding: .utf8).split(separator: "\n")
var times = lines.map { l -> (Double, String) in
    let parts = l.split(separator: "\t", maxSplits: 1, omittingEmptySubsequences: false)
    return (Double(parts[0])!, parts.count > 1 ? String(parts[1]) : "")
}
let scale = seconds / times.last!.0
times = times.map { ($0.0 * scale, $0.1) }
try FileManager.default.createDirectory(atPath: outDir, withIntermediateDirectories: true)

// Input 0 is the video; input i is caption i-1, shown until the next one.
var filter = "[0:v]pad=iw:ih+\(stripH):0:0:color=0x\(String(mantle, radix: 16))[v0]"
for (i, (start, caption)) in times.dropLast().enumerated() {
    draw(caption, to: "\(outDir)/\(String(format: "%02d", i)).png")
    let end = times[i + 1].0
    filter += String(format: ";[v%d][%d:v]overlay=0:H-h:enable='between(t,%.2f,%.2f)'[v%d]", i, i + 1, start, end, i + 1)
}
filter += ";[v\(times.count - 1)]null[out]"
try filter.write(toFile: "\(outDir)/filter.txt", atomically: true, encoding: .utf8)
