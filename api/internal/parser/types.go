package parser

// Document — распарсенный документ НПА.
type Document struct {
	Title    string    // название документа если удалось извлечь
	Sections []Section // верхний уровень иерархии
}

// Section — секция документа (глава, статья, пункт, часть).
type Section struct {
	ID       string    // уникальный ID внутри документа: "s1", "s1.2", "s1.2.3"
	Label    string    // метка как в тексте: "Статья 1", "Глава 2", "п. 3.1"
	Text     string    // чистый текст секции без детей
	Level    int       // глубина вложенности: 0=глава, 1=статья, 2=пункт, 3=часть
	Children []Section // вложенные секции
}

// Flatten возвращает все секции документа в плоском списке (DFS).
// Удобно для эмбеддинга и индексации в Qdrant.
func (d *Document) Flatten() []Section {
	var result []Section
	var walk func(sections []Section)
	walk = func(sections []Section) {
		for _, s := range sections {
			result = append(result, s)
			walk(s.Children)
		}
	}
	walk(d.Sections)
	return result
}

// FlattenLeaves возвращает только листовые секции (без детей).
// Используется для чанкования при индексации в Qdrant.
func (d *Document) FlattenLeaves() []Section {
	var result []Section
	var walk func(sections []Section)
	walk = func(sections []Section) {
		for _, s := range sections {
			if len(s.Children) == 0 {
				result = append(result, s)
			} else {
				walk(s.Children)
			}
		}
	}
	walk(d.Sections)
	return result
}
