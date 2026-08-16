export function dataUrlToBlob(dataUrl: string) {
    const separator = dataUrl.indexOf(",");
    if (!dataUrl.startsWith("data:") || separator < 0) throw new Error("无效的 data URL");

    const metadata = dataUrl.slice(5, separator);
    const content = dataUrl.slice(separator + 1);
    const mimeType = metadata.split(";", 1)[0] || "application/octet-stream";
    let bytes: Uint8Array;
    if (metadata.toLowerCase().split(";").includes("base64")) {
        const decoded = atob(content);
        bytes = new Uint8Array(decoded.length);
        for (let index = 0; index < decoded.length; index += 1) bytes[index] = decoded.charCodeAt(index);
    } else {
        bytes = new TextEncoder().encode(decodeURIComponent(content));
    }
    const buffer = bytes.buffer instanceof ArrayBuffer ? bytes.buffer : new Uint8Array(bytes).buffer;
    return new Blob([buffer], { type: mimeType });
}

export async function sourceToBlob(source: string) {
    if (source.startsWith("data:")) return dataUrlToBlob(source);
    const response = await fetch(source);
    if (!response.ok) throw new Error(`读取媒体失败（HTTP ${response.status}）`);
    return response.blob();
}
