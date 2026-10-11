import re

from .types import InvalidInput


def validate_strategy(options=None):
    options = {} if options is None else options
    if not isinstance(options, dict):
        raise InvalidInput('Invalid strategy')
    strategy = options.get('strategy', 'fixed')
    size = options.get('size', 800)
    if type(size) is not int or size <= 0:
        raise InvalidInput('Invalid size')
    overlap = options.get('overlap', size // 10)
    if type(overlap) is not int or not 0 <= overlap < size:
        raise InvalidInput('Invalid overlap')
    if strategy not in ('fixed', 'delimiter', 'markdown'):
        raise InvalidInput('Invalid strategy')
    delimiter = options.get('delimiter')
    if strategy == 'delimiter' and (not isinstance(delimiter, str) or not delimiter):
        raise InvalidInput('Invalid delimiter')
    return dict(strategy=strategy, size=size, overlap=overlap, delimiter=delimiter)


def chunks(text, options=None):
    opts = validate_strategy(options)
    sections = [text]
    if opts['strategy'] == 'delimiter':
        delimiter = opts['delimiter']
        parts = text.split(delimiter)
        sections = [part + (delimiter if i < len(parts) - 1 else '') for i, part in enumerate(parts)]
    elif opts['strategy'] == 'markdown':
        sections, lines, fence = [], [], None
        for line in text.splitlines(keepends=True):
            marker = re.match(r'^ {0,3}(`{3,}|~{3,})', line)
            if fence:
                if re.match(r'^ {0,3}' + re.escape(fence[0]) + '{' + str(len(fence)) + r',}\s*$', line):
                    fence = None
            elif marker:
                fence = marker[1]
            elif re.match(r'^#{1,6}(?:\s|$)', line) and lines:
                sections.append(''.join(lines))
                lines = []
            lines.append(line)
        sections.append(''.join(lines))
    result = []
    for section in sections:
        for start in range(0, len(section), opts['size'] - opts['overlap']):
            piece = section[start:start + opts['size']]
            if piece.strip():
                result.append(piece)
            if start + opts['size'] >= len(section):
                break
    return result
