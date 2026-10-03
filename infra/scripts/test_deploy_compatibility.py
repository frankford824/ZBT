import unittest
from pathlib import Path
from deploy_dev import migration_compatible


class AdditiveMigrationTests(unittest.TestCase):
    def test_exact_reviewed_function_is_compatible_with_old_images(self):
        name = '00038_authenticated_login_tenants.sql'
        sql = (Path(__file__).resolve().parents[2] / 'backend/internal/db/migrations' / name).read_bytes()
        self.assertTrue(migration_compatible({'old.sql': b'old'}, {'old.sql': b'old', name: sql}))
        self.assertFalse(migration_compatible({'old.sql': b'old'}, {'old.sql': b'old', name: sql + b'\n'}))

    def test_rewrite_removal_and_unreviewed_migration_are_blocked(self):
        for candidate in ({}, {'old.sql': b'changed'}, {'old.sql': b'old', 'other.sql': b'create table test()'}):
            self.assertFalse(migration_compatible({'old.sql': b'old'}, candidate))
