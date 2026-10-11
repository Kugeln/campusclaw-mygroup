from flask import Blueprint, current_app, g, jsonify, render_template, request, send_file
from pathlib import Path
import sqlite3

from app.auth import authenticated, teacher_required
from app.storage.materials import find, listing, persist
from app.materials import prepare_upload
from app.retrieval.types import Unavailable, Scope

bp = Blueprint('materials', __name__)


@bp.get('/materials')
def materials_page():
    return render_template('materials.html')


@bp.get('/api/materials')
@authenticated()
def material_list():
    if request.args:
        return jsonify(error='Client scope parameters are not accepted'), 400
    return jsonify(items=[dict(row) for row in listing(g.principal.class_id)])


@bp.post('/api/materials')
@authenticated()
@teacher_required
def material_upload():
    if request.args or request.form or any(field != 'file' for field in request.files):
        return jsonify(error='Client identity fields are not accepted'), 400
    if len(request.files.getlist('file')) != 1:
        return jsonify(error='Exactly one file is required'), 400
    file = request.files['file']
    root = current_app.config['UPLOAD_ROOT']
    try:
        with prepare_upload(root, file, current_app.config['MAX_UPLOAD_BYTES']) as (temporary, text):
            material_id = persist(root, file.filename, temporary, text, g.principal.class_id, g.principal.user_id)
    except (OSError, sqlite3.Error, Unavailable):
        return jsonify(error='Material storage unavailable'), 500
    if 'indexing' in current_app.extensions:
        p = g.principal
        try:
            status = current_app.extensions['indexing'].run(Scope(p.user_id, p.role, p.class_id), material_id, fresh=True)
        except Unavailable:
            status = 'failed'
        return jsonify(material_id=material_id, index_status=status), 201
    return jsonify(material_id=material_id), 201


@bp.get('/api/materials/<material_id>')
@authenticated()
def material_detail(material_id):
    material = find(material_id, g.principal.class_id)
    if material is None:
        return jsonify(error='Material not found'), 404
    keys = ['id', 'original_name', 'size', 'created_at', 'uploader', 'plain_text']
    keys.extend(key for key in ('index_status', 'cleanup_required') if key in material.keys())
    return jsonify({key: material[key] for key in keys})


@bp.get('/api/materials/<material_id>/file')
@authenticated()
def material_file(material_id):
    material = find(material_id, g.principal.class_id)
    if material is None:
        return jsonify(error='Material not found'), 404
    response = send_file(Path(current_app.config['UPLOAD_ROOT']) / 'files' / material['storage_key'], mimetype='text/plain', as_attachment=True, download_name=material['original_name'])
    response.headers['X-Content-Type-Options'] = 'nosniff'
    response.headers['Cache-Control'] = 'private, no-store'
    return response
