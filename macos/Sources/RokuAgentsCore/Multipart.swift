import Foundation

/// A multipart/form-data body, as the Roku developer installer expects.
public struct MultipartForm: Sendable {
    public let boundary: String

    public init(boundary: String = "rokuagents-\(UUID().uuidString)") {
        self.boundary = boundary
    }

    public var contentType: String { "multipart/form-data; boundary=\(boundary)" }

    public struct File: Sendable {
        public let field: String
        public let filename: String
        public let contentType: String
        public let data: Data

        public init(field: String, filename: String, contentType: String, data: Data) {
            self.field = field
            self.filename = filename
            self.contentType = contentType
            self.data = data
        }
    }

    public func body(fields: [(String, String)], file: File) -> Data {
        var body = Data()
        func append(_ text: String) { body.append(Data(text.utf8)) }
        for (name, value) in fields {
            append("--\(boundary)\r\n")
            append("Content-Disposition: form-data; name=\"\(name)\"\r\n\r\n")
            append("\(value)\r\n")
        }
        append("--\(boundary)\r\n")
        append("Content-Disposition: form-data; name=\"\(file.field)\"; filename=\"\(file.filename)\"\r\n")
        append("Content-Type: \(file.contentType)\r\n\r\n")
        body.append(file.data)
        append("\r\n--\(boundary)--\r\n")
        return body
    }
}
