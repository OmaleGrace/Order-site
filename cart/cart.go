package cart

type Cart struct {
	Items []int
}

func (c *Cart) Add(itemID int) {
	c.Items = append(c.Items, itemID)
}
