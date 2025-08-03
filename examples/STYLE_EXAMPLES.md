# Clicky Style Examples

This document demonstrates the new styling capabilities in Clicky using Tailwind CSS classes.

## Style Fields

Clicky now supports four style-related fields in schemas:

1. **`style`** - Applies to field values (highest priority)
2. **`label_style`** - Applies to field labels/names
3. **`header_style`** - Applies to table headers  
4. **`row_style`** - Applies to all table rows (lowest priority)

### Style Priority

When multiple styles could apply, they are prioritized as follows:

1. **Individual field `style`** (highest priority) - overrides everything
2. **Table `row_style`** - applies when no field style is specified
3. **Color from `color_options`** based on value
4. **Default format-based styling** (lowest priority)

**Important:** Column styles always override row styles in tables!

## Supported Tailwind Classes

### Text Colors
- Basic colors: `text-red-500`, `text-blue-700`, `text-green-600`
- Shades from 50-950: `text-gray-50` to `text-gray-950`
- All Tailwind colors: slate, gray, zinc, neutral, stone, red, orange, amber, yellow, lime, green, emerald, teal, cyan, sky, blue, indigo, violet, purple, fuchsia, pink, rose

### Background Colors
- Format: `bg-{color}-{shade}`
- Examples: `bg-blue-100`, `bg-green-50`, `bg-red-900`

### Typography

#### Font Weights
- `font-thin` - Thin text (rendered as faint in terminal)
- `font-extralight` - Extra light text (rendered as faint)
- `font-light` - Light text (rendered as faint)
- `font-normal` - Normal weight
- `font-medium` - Medium weight (rendered as bold in terminal)
- `font-semibold` - Semi-bold text (rendered as bold in terminal)
- `font-bold` - Bold text

#### Font Styles
- `italic` - Italic text
- `not-italic` - Remove italic

#### Text Decoration
- `underline` - Underlined text
- `no-underline` - Remove underline
- `line-through` or `strikethrough` - Strikethrough text
- `overline` - Overline (rendered as underline in terminal)

#### Text Transform (NEW!)
- `uppercase` - Transform text to UPPERCASE
- `lowercase` - Transform text to lowercase
- `capitalize` - Capitalize Each Word
- `normal-case` - Preserve original casing

#### Font Family
- `font-mono` - Monospace font

### Text Sizes
- `text-xs`, `text-sm`, `text-base`, `text-lg`, `text-xl`, `text-2xl`, `text-3xl`

### Spacing (for display in HTML/web contexts)
- `px-2`, `py-1` - Padding
- `mx-2`, `my-1` - Margin
- `p-2`, `m-2` - All-sides padding/margin

### Visibility & Opacity
- `visible` - Make element visible
- `invisible` - Make element invisible (faint in terminal)
- `opacity-25` - 25% opacity (rendered as faint)
- `opacity-50` - 50% opacity (rendered as faint)
- `opacity-75` - 75% opacity (rendered as faint)
- `opacity-100` - Full opacity

### Other Effects
- `rounded` - Rounded corners (for web/HTML)
- `rounded-full` - Fully rounded (for web/HTML)
- `tracking-wider` - Letter spacing (for web/HTML)
- `truncate` - Truncate text with ellipsis
- `text-ellipsis` - Add ellipsis to overflowing text
- `text-clip` - Clip overflowing text

## Example: Label Styling

Label styles control how field names/labels appear:

```yaml
fields:
  # Label will be blue and bold, value will be green
  - name: "username"
    type: "string"
    label_style: "text-blue-600 font-bold uppercase"
    style: "text-green-600"
    
  # Label will be small and gray, value will be large and emphasized
  - name: "email"
    type: "string"
    label_style: "text-gray-500 text-xs"
    style: "text-xl font-semibold text-purple-700"
```

## Example: Text Transform

Text transforms are applied to the actual text content, changing how it appears:

```yaml
fields:
  # Will display as "JOHN DOE" even if value is "john doe"
  - name: "name"
    type: "string"
    style: "uppercase text-blue-600 font-bold"
    label_style: "text-blue-400 text-sm"
    
  # Will display as "active" even if value is "ACTIVE"
  - name: "status"
    type: "string"
    style: "lowercase text-green-500"
    label_style: "text-green-700 font-medium"
    
  # Will display as "Hello World" even if value is "hello world"
  - name: "title"
    type: "string"
    style: "capitalize text-purple-700 font-semibold"
    label_style: "text-purple-500 uppercase tracking-wide"
```

## Example: Basic Field Styling

```yaml
fields:
  - name: "username"
    type: "string"
    style: "text-blue-600 font-bold"
    
  - name: "email"
    type: "string"
    style: "text-gray-600 italic"
    
  - name: "status"
    type: "string"
    style: "bg-green-100 text-green-800 px-2 py-1 rounded"
```

## Example: Table with Style Priority

```yaml
fields:
  - name: "transactions"
    type: "array"
    format: "table"
    table_options:
      header_style: "bg-blue-100 text-blue-900 font-bold uppercase"
      row_style: "text-gray-700 hover:bg-gray-50"  # Applied to all cells
      fields:
        - name: "date"
          type: "string"
          format: "date"
          style: "text-blue-500 font-mono"  # OVERRIDES row_style
        - name: "amount"
          type: "float"
          format: "currency"
          style: "text-green-600 font-bold"  # OVERRIDES row_style
        - name: "status"
          type: "string"
          # NO style specified - uses row_style
        - name: "description"
          type: "string"
          style: "capitalize text-purple-600"  # OVERRIDES row_style
```

In this example:
- Headers get `header_style` styling
- `date` and `amount` columns override `row_style` with their own styles
- `status` column uses the `row_style` since no individual style is specified
- `description` column overrides `row_style` with capitalize and purple color

## Example: Combining Styles with Color Options

```yaml
fields:
  - name: "priority"
    type: "string"
    style: "font-bold uppercase text-xs"
    color_options:
      red: "critical"
      yellow: "medium"
      green: "low"
```

In this example:
- The field always has `font-bold uppercase text-xs` styling
- The color changes based on the value (red for "critical", etc.)

## Example: Nested Struct with Styles

```yaml
fields:
  - name: "user"
    type: "struct"
    style: "bg-gray-50 p-4 rounded"
    fields:
      - name: "name"
        type: "string"
        style: "text-indigo-700 font-semibold text-lg"
      - name: "role"
        type: "string"
        style: "text-gray-500 italic"
```

## Example: Advanced Table Styling

```yaml
fields:
  - name: "metrics"
    type: "array"
    format: "table"
    table_options:
      title: "Performance Metrics"
      header_style: "bg-gradient-to-r from-purple-600 to-indigo-600 text-white font-bold"
      row_style: "text-gray-700 border-b border-gray-200 hover:bg-purple-50"
      fields:
        - name: "metric"
          type: "string"
          style: "font-medium text-gray-900"
        - name: "value"
          type: "float"
          style: "font-mono text-right"
        - name: "change"
          type: "float"
          style: "font-bold text-right"
          color_options:
            green: ">=0"
            red: "<0"
```

## Style Priority

When multiple styles could apply, they are prioritized as follows:

1. Individual field `style` (highest priority)
2. Table `row_style` 
3. Color from `color_options` based on value
4. Default format-based styling (lowest priority)

## Using in Struct Tags

You can also apply styles directly in Go struct tags:

```go
type User struct {
    Name     string  `pretty:"string,style=text-blue-600 font-bold"`
    Email    string  `pretty:"string,style=text-gray-600"`
    Status   string  `pretty:"string,style=font-semibold"`
    Balance  float64 `pretty:"currency,style=text-green-600 font-bold"`
}

type Report struct {
    Title string      `pretty:"string,style=text-2xl font-bold"`
    Items []Item      `pretty:"table,header_style=bg-blue-100 text-blue-900,row_style=text-gray-700"`
}
```

## Output Format Support

Clicky's styling system now supports multiple output formats:

### Terminal Output
- **Supported**: Colors, bold, italic, underline, text transforms
- **Text transforms**: Applied to actual text content (uppercase, lowercase, capitalize)
- **Fallbacks**: Complex styles gracefully degrade to basic formatting

### PDF Output  
- **Supported**: Colors (hex), bold, italic, text transforms
- **Font styling**: Bold and italic combinations supported
- **Color conversion**: Tailwind colors automatically converted to RGB
- **Limitations**: No underline/strikethrough, background colors require custom implementation

### HTML Output
- **Supported**: All Tailwind CSS classes via CDN
- **Full compatibility**: Spacing, borders, gradients, animations, responsive classes
- **Text transforms**: Applied to content before HTML generation
- **Best experience**: HTML output provides the richest styling capabilities

### Recommended Usage
- **Simple styles**: Use basic colors and text formatting for cross-format compatibility
- **Advanced styles**: Use complex Tailwind classes when primarily targeting HTML output
- **Text transforms**: Work consistently across all formats

## Complete Schema Examples

See the following example schemas for comprehensive demonstrations:

1. **comprehensive-styling-demo.yaml** - Complete showcase of all styling features
2. **order-schema.yaml** - E-commerce order with styled fields and tables
3. **user-profile-schema.yaml** - User profile with various text styles
4. **dashboard-metrics-schema.yaml** - Metrics dashboard with gradient headers
5. **project-tasks-schema.yaml** - Project management with rich styling
6. **text-transform-demo.yaml** - Text transformation examples

## Tips for Effective Styling

1. **Consistency**: Use consistent colors for similar data types
2. **Hierarchy**: Use font sizes and weights to establish visual hierarchy
3. **Readability**: Don't overuse colors; maintain good contrast
4. **Semantic Colors**: Use green for success, red for errors, yellow for warnings
5. **Combine Classes**: Mix multiple classes for complex styling: `"text-blue-600 font-bold uppercase"`

## Default Styles

If no style is specified, Clicky applies sensible defaults:
- Headers: Bold with primary theme color
- Currency: Green color
- Dates: Default text color
- Tables: Alternating row colors for readability