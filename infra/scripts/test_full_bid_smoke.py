import unittest
from full_bid_smoke import export_body_samples, review_fixture_text, submission_date_used_as_delivery


class FixtureReviewTests(unittest.TestCase):
    def test_delivery_check_does_not_cross_sentence_or_clause_boundaries(self):
        for text in ('确保按期完工。投标截止时间为2026年11月15日09:30，仅作为投标文件递交的截止时间，并非施工交付时间。',
                     '合同生效后30天交付，投标截止2026-11-15 09:30。'):
            self.assertFalse(submission_date_used_as_delivery(text))
        for text in ('承诺交付日期为2026-11-15。', '计划在完工日期2026年11月15日前完成施工。', '竣工：2026-11-15。'):
            self.assertTrue(submission_date_used_as_delivery(text))

    def test_export_samples_allow_word_numbering_without_dropping_body_numbers(self):
        text = '5. 交付保障\n预算128万元，交付30天。\n6. 结语\n保证工程质量、安全、进度和成本控制，为采购单位提供优质服务。'
        samples = export_body_samples(text)
        self.assertIn('预算128万元交付30天', samples)
        self.assertIn('结语', samples)
        self.assertNotIn('6结语', samples)
        exported = '交付保障预算128万元交付30天结语保证工程质量安全进度和成本控制为采购单位提供优质服务'
        self.assertTrue(all(sample in exported for sample in samples))
        self.assertFalse(all(sample in exported.replace('128', '100') for sample in samples))
        self.assertFalse(all(sample in exported.replace('保证工程质量', '') for sample in samples))

    def test_export_samples_keep_decimal_subheadings_and_dates(self):
        self.assertEqual(export_body_samples('1.1 资格要求\n2026-11-15 09:30'), ['11资格要求', '202611150930'])

    def test_fixture_review_removes_promises_without_inventing_replacements(self):
        text = '项目预算128万元。\n【事实待核实：质保承诺缺少依据。】'
        reviewed = review_fixture_text(text)
        self.assertIn('项目预算128万元。', reviewed)
        self.assertIn('相关承诺未作出', reviewed)
        self.assertNotIn('【事实待核实：', reviewed)
        self.assertNotIn('12个月', reviewed)

    def test_manual_fixture_review_does_not_touch_supported_text(self):
        text = '投标截止时间2026-11-15 09:30，合同生效后30天内交付。'
        self.assertEqual(review_fixture_text(text), text)
