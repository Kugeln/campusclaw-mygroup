"""Account provisioning shared by explicit commands and seed initialization."""

from werkzeug.security import generate_password_hash

SAMPLE_ACCOUNTS = (
    ('teacher_a', 'teacher', 'A', 'SEED_TEACHER_A_PASSWORD'),
    ('student_a1', 'student', 'A', 'SEED_STUDENT_A1_PASSWORD'),
    ('teacher_b', 'teacher', 'B', 'SEED_TEACHER_B_PASSWORD'),
    ('student_b1', 'student', 'B', 'SEED_STUDENT_B1_PASSWORD'),
)


def create_class(db, name):
    if not name:
        raise ValueError('Class name is required')
    db.execute('INSERT INTO classes(name) VALUES (?) ON CONFLICT(name) DO NOTHING', (name,))
    return db.execute('SELECT id FROM classes WHERE name = ?', (name,)).fetchone()[0]


def create_user(db, username, password, role, class_id):
    if not username or not password:
        raise ValueError('Username and password are required')
    return db.execute(
        'INSERT INTO users(username,password_hash,role,class_id) VALUES (?,?,?,?)',
        (username, generate_password_hash(password, method='pbkdf2:sha256:1000000'), role, class_id),
    ).lastrowid


def seed_accounts(db, password_provider):
    for username, role, class_name, variable in SAMPLE_ACCOUNTS:
        class_id = create_class(db, class_name)
        if not db.execute('SELECT id FROM users WHERE username = ?', (username,)).fetchone():
            create_user(db, username, password_provider(variable), role, class_id)
