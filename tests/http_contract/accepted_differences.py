"""User-approved Phase 5 base-material boundaries; no RAG or wildcard rules.

Each listed case still validates its backend-specific response. A failing
assertion is never converted into acceptance by this registry.
"""
MATERIAL = '/api/materials/11111111-1111-4111-8111-111111111111'
FILE = '/api/materials/33333333-3333-4333-8333-333333333333/file'
METHOD_PATHS = {
    '/api/materials': ['GET', 'HEAD', 'POST'],
    MATERIAL: ['GET', 'HEAD'],
    MATERIAL+'/file': ['GET', 'HEAD'],
    '/api/auth/login': ['POST'],
    '/login': ['GET', 'HEAD'],
}
RULES = {
    'query.&': 'empty-query-separator',
    'upload.oversize_then_identity': 'file-limit-before-identity',
    'multipart.large_form_field': 'identity-before-form-limit',
    **{'method.'+method+'.'+path:'accurate-method-capabilities'
       for method in ('PUT','PATCH','DELETE','OPTIONS') for path in METHOD_PATHS},
    **{'validator.'+kind+'.'+path:'optional-response-validators'
       for kind in ('headers','If-None-Match','If-Modified-Since') for path in (FILE,'/static/logo.png')},
}
assert len(RULES) == 29


def expected_status(case, backend, default):
    overrides = {
        'query.&': {'Flask':200,'Go':400},
        'upload.oversize_then_identity': {'Flask':400,'Go':413},
        'multipart.large_form_field': {'Flask':413,'Go':400},
        **{'method.OPTIONS.'+path:{'Flask':200,'Go':405} for path in METHOD_PATHS},
        **{'validator.'+kind+'.'+path:{'Flask':304,'Go':200}
           for kind in ('If-None-Match','If-Modified-Since') for path in (FILE,'/static/logo.png')},
    }
    return overrides.get(case,{}).get(backend,default)


def assert_response(case, backend, response):
    """Validate exact JSON/media for the three distinct input boundaries."""
    expected = {
        ('query.&','Flask'):('application/json',None), # exact scoped items checked by case
        ('query.&','Go'):('application/json',{'error':'Client scope parameters are not accepted'}),
        ('upload.oversize_then_identity','Flask'):('application/json',{'error':'Client identity fields are not accepted'}),
        ('upload.oversize_then_identity','Go'):('text/html',None),
        ('multipart.large_form_field','Flask'):('text/html',None),
        ('multipart.large_form_field','Go'):('application/json',{'error':'Client identity fields are not accepted'}),
    }
    media, body = expected[(case,backend)]
    assert response.headers.get('content-type','').split(';')[0] == media, 'approved boundary media changed'
    assert response.headers.get('cache-control') == 'no-store', 'API cache policy changed'
    assert response.headers.get('x-content-type-options') == 'nosniff', 'API nosniff changed'
    if body is not None:
        assert response.json() == body, 'approved boundary JSON changed'


def expected_allow(path, backend):
    return sorted(METHOD_PATHS[path] + (['OPTIONS'] if backend=='Flask' else []))
