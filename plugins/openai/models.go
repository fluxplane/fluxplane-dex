package openai

type OpenAITargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"required,description=Registered OpenAI endpoint ref resolved by the host."`
}

type ImageGenerateInput struct {
	OpenAITargetInput
	Prompt            string `json:"prompt,omitempty" jsonschema:"required,description=Text description of the desired image. Max 32000 chars for GPT image models\\, 4000 for dall-e-3\\, 1000 for dall-e-2."`
	Model             string `json:"model,omitempty" jsonschema:"description=Image model. Known values: gpt-image-1\\, gpt-image-1-mini\\, gpt-image-1.5\\, dall-e-3\\, dall-e-2. Custom values accepted."`
	N                 int    `json:"n,omitempty" jsonschema:"description=Number of images to generate. dall-e-3 only supports 1.,minimum=1,maximum=10"`
	Size              string `json:"size,omitempty" jsonschema:"description=Image dimensions. Standard: auto\\, 1024x1024\\, 1536x1024\\, 1024x1536\\, 256x256\\, 512x512\\, 1792x1024\\, 1024x1792. GPT image models also accept arbitrary WxH (divisible by 16\\, aspect between 1:3 and 3:1)."`
	Quality           string `json:"quality,omitempty" jsonschema:"description=Image quality. GPT image: low|medium|high|auto. dall-e-3: hd|standard. dall-e-2: standard.,enum=auto,enum=high,enum=medium,enum=low,enum=hd,enum=standard"`
	Style             string `json:"style,omitempty" jsonschema:"description=Image style. dall-e-3 only.,enum=vivid,enum=natural"`
	ResponseFormat    string `json:"response_format,omitempty" jsonschema:"description=Response format for dall-e models. GPT image models always return b64_json.,enum=url,enum=b64_json"`
	OutputFormat      string `json:"output_format,omitempty" jsonschema:"description=Output format for GPT image models.,enum=png,enum=jpeg,enum=webp"`
	Background        string `json:"background,omitempty" jsonschema:"description=Background for GPT image models. transparent requires png or webp output_format.,enum=transparent,enum=opaque,enum=auto"`
	Moderation        string `json:"moderation,omitempty" jsonschema:"description=Content moderation level for GPT image models.,enum=low,enum=auto"`
	OutputCompression int    `json:"output_compression,omitempty" jsonschema:"description=Compression level for GPT image jpeg/webp.,minimum=0,maximum=100"`
	User              string `json:"user,omitempty" jsonschema:"description=End-user identifier passed to OpenAI for abuse monitoring."`
}

type ImageGenerateResult struct {
	Created      int64            `json:"created" jsonschema:"description=Unix timestamp (seconds) when the response was created."`
	Data         []GeneratedImage `json:"data" jsonschema:"description=Generated images."`
	Background   string           `json:"background,omitempty" jsonschema:"description=Background used. GPT image models only.,enum=transparent,enum=opaque"`
	OutputFormat string           `json:"output_format,omitempty" jsonschema:"description=Output format used. GPT image models only.,enum=png,enum=webp,enum=jpeg"`
	Size         string           `json:"size,omitempty" jsonschema:"description=Image size used."`
	Quality      string           `json:"quality,omitempty" jsonschema:"description=Image quality used. GPT image models only.,enum=low,enum=medium,enum=high"`
	Usage        *ImageGenUsage   `json:"usage,omitempty" jsonschema:"description=Token usage. GPT image models only."`
}

type GeneratedImage struct {
	URL           string `json:"url,omitempty" jsonschema:"description=URL of the generated image. dall-e models with response_format=url. Valid for 60 minutes."`
	B64JSON       string `json:"b64_json,omitempty" jsonschema:"description=Base64-encoded image. Default for GPT image models; set response_format=b64_json for dall-e."`
	RevisedPrompt string `json:"revised_prompt,omitempty" jsonschema:"description=Revised prompt actually used. dall-e-3 only."`
}

type ImageGenUsage struct {
	TotalTokens        int                       `json:"total_tokens,omitempty" jsonschema:"description=Total tokens consumed."`
	InputTokens        int                       `json:"input_tokens,omitempty" jsonschema:"description=Tokens in the input prompt."`
	OutputTokens       int                       `json:"output_tokens,omitempty" jsonschema:"description=Image tokens in the output."`
	InputTokensDetails *ImageGenInputTokenDetail `json:"input_tokens_details,omitempty" jsonschema:"description=Breakdown of input tokens."`
}

type ImageGenInputTokenDetail struct {
	TextTokens  int `json:"text_tokens,omitempty" jsonschema:"description=Text tokens in the input prompt."`
	ImageTokens int `json:"image_tokens,omitempty" jsonschema:"description=Image tokens in the input prompt."`
}

type responsesOutput struct {
	ID         string            `json:"id,omitempty"`
	Model      string            `json:"model,omitempty"`
	OutputText string            `json:"output_text,omitempty"`
	Output     []responseMessage `json:"output,omitempty"`
	Usage      map[string]any    `json:"usage,omitempty"`
}

type responseMessage struct {
	Type    string            `json:"type,omitempty"`
	Role    string            `json:"role,omitempty"`
	Content []responseContent `json:"content,omitempty"`
}

type responseContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type ModelListInput struct {
	OpenAITargetInput
}

type Model struct {
	ID      string `json:"id" jsonschema:"description=Model identifier (e.g. gpt-4o\\, dall-e-3\\, gpt-image-1)."`
	Object  string `json:"object,omitempty" jsonschema:"description=Object type. Always 'model'.,enum=model"`
	Created int64  `json:"created,omitempty" jsonschema:"description=Unix timestamp (seconds) when the model was created."`
	OwnedBy string `json:"owned_by,omitempty" jsonschema:"description=Organization that owns the model (e.g. 'openai'\\, 'system')."`
}
