import ReactMarkdown from "react-markdown";

/** Renders Markdown without raw HTML (react-markdown escapes it) and with safe links. */
export function Markdown({ children }: { children: string }) {
  return (
    <div className="prose-sans">
      <ReactMarkdown
        components={{
          a: ({ href, children }) => (
            <a href={href} target="_blank" rel="noopener noreferrer nofollow">
              {children}
            </a>
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
