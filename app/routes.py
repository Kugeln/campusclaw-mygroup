from flask import Blueprint, current_app, jsonify, render_template, request, send_file, session
from pathlib import Path
import sqlite3

from app.auth import authenticated, teacher_required
from app.csrf import csrf_required, csrf_token
from app.db import find_material, get_db, list_materials
from app.materials import persist_prepared, prepare_upload

bp = Blueprint('materials', __name__)


@bp.get('/materials')
@authenticated(page=True)
def materials_page():
    current_class = get_db().execute('SELECT name FROM classes WHERE id=?', (session['class_id'],)).fetchone()
    return render_template('materials.html', items=list_materials(get_db(), session['class_id']), role=session['role'], class_name=current_class['name'], csrf_token=csrf_token())


@bp.get('/api/materials')
@authenticated()
def material_list():
    if request.args:
        return jsonify(error='Client scope parameters are not accepted'), 400
    return jsonify(items=[dict(row) for row in list_materials(get_db(), session['class_id'])])


@bp.post('/api/materials')
@authenticated()
@teacher_required
@csrf_required
def material_upload():
    if request.args or any(field != 'csrf_token' for field in request.form) or any(field != 'file' for field in request.files):
        return jsonify(error='Client identity fields are not accepted'), 400
    if len(request.files.getlist('file')) != 1:
        return jsonify(error='Exactly one file is required'), 400
    file = request.files['file']
    root = current_app.config['UPLOAD_ROOT']
    try:
        with prepare_upload(root, file, current_app.config['MAX_UPLOAD_BYTES']) as (temporary, text):
            material_id = persist_prepared(get_db(), root, file.filename, temporary, text, session['class_id'], session['user_id'])
    except (OSError, sqlite3.Error):
        return jsonify(error='Material storage unavailable'), 500
    return jsonify(material_id=material_id), 201


@bp.get('/api/materials/<material_id>')
@authenticated()
def material_detail(material_id):
    material = find_material(get_db(), material_id, session['class_id'])
    if material is None:
        return jsonify(error='Material not found'), 404
    return jsonify({key: material[key] for key in ('id', 'original_name', 'size', 'created_at', 'uploader', 'plain_text')})


@bp.get('/api/materials/<material_id>/file')
@authenticated()
def material_file(material_id):
    material = find_material(get_db(), material_id, session['class_id'])
    if material is None:
        return jsonify(error='Material not found'), 404
    response = send_file(Path(current_app.config['UPLOAD_ROOT']) / 'files' / material['storage_key'], mimetype='text/plain', as_attachment=True, download_name=material['original_name'])
    response.headers['X-Content-Type-Options'] = 'nosniff'
    response.headers['Cache-Control'] = 'private, no-store'
    return response
